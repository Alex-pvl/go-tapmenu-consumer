package tapmenu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/alex-pvl/go-tapmenu-consumer/internal/app/config"
	"github.com/alex-pvl/go-tapmenu-consumer/internal/app/store"
	"github.com/alex-pvl/go-tapmenu-consumer/internal/app/tapmenu-consumer/kafka"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Server struct {
	configuration *config.Configuration
	logger        *logrus.Logger
	router        *mux.Router
	db            *store.Store
	consumer      *kafka.Consumer
	mu            sync.RWMutex
	orders        []*store.Order
	stopCron      chan struct{}
}

func New(
	configuration *config.Configuration,
	db *store.Store,
	consumer *kafka.Consumer,
	logger *logrus.Logger,
) *Server {
	return &Server{
		configuration: configuration,
		logger:        logger,
		router:        mux.NewRouter(),
		db:            db,
		consumer:      consumer,
		orders:        make([]*store.Order, 0),
		stopCron:      make(chan struct{}),
	}
}

func (s *Server) Start() error {
	s.configureRouter()

	go s.startConsumer()
	go s.startOrderCleanupCron()

	s.logger.Infof("starting server on %s", s.configuration.BindAddress)

	handler := s.corsMiddleware(s.router)

	return http.ListenAndServe(s.configuration.BindAddress, handler)
}

func (s *Server) configureRouter() {
	s.router.HandleFunc("/login", s.handleLogin()).Methods(http.MethodPost)
	s.router.Handle("/orders", s.authMiddleware(s.handleGetOrders())).Methods(http.MethodGet)
	s.router.Handle("/orders/{orderId}/accept", s.authMiddleware(s.handleAcceptOrder())).Methods(http.MethodPost)
}

func (s *Server) handleGetOrders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageParam := r.URL.Query().Get("page")
		sizeParam := r.URL.Query().Get("size")

		page, _ := strconv.Atoi(pageParam)
		size, _ := strconv.Atoi(sizeParam)

		if page < 1 {
			page = 1
		}
		if size < 1 {
			size = 10
		}

		waiter, err := s.getWaiter(r.Context())
		if err != nil {
			s.logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.mu.RLock()
		defer s.mu.RUnlock()

		s.orders, err = s.db.GetOrderSliceByRestaurant(waiter.RestaurantName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		total := len(s.orders)
		start := (page - 1) * size
		if start > total {
			start = total
		}
		end := start + size
		if end > total {
			end = total
		}

		response := map[string]interface{}{
			"page":  page,
			"size":  size,
			"total": total,
			"data":  s.orders[start:end],
		}

		renderJSON(w, response)
	}
}

func (s *Server) handleAcceptOrder() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorMsg string
		vars := mux.Vars(r)
		orderId := vars["orderId"]
		orderIdParsed, err := uuid.Parse(orderId)
		if err != nil {
			s.logger.Errorf("error parsing order id %s", orderId)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		found := false

		for i := range s.orders {
			if s.orders[i].Id == orderIdParsed {
				s.orders[i].Accepted = true
				found = true
				break
			}
		}

		if !found {
			errorMsg = "order [" + orderId + "] not found"
			s.logger.Error(errorMsg)
			http.Error(w, errorMsg, http.StatusNotFound)
			return
		}

		order, err := s.db.GetOrder(uuid.MustParse(orderId))
		if err != nil {
			s.logger.Errorf("error getting order [%s]", orderId)
			http.Error(w, "error getting order: "+orderId, http.StatusInternalServerError)
			return
		}

		waiter, err := s.getWaiter(r.Context())
		if err != nil {
			s.logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if waiter.RestaurantName != order.RestaurantName {
			errorMsg = fmt.Sprintf("waiter %s[%s] is not from [%s] restaurant", waiter.Username, waiter.Id, order.RestaurantName)
			s.logger.Error(errorMsg)
			http.Error(w, errorMsg, http.StatusBadRequest)
			return
		}

		if order.Accepted {
			errorMsg = fmt.Sprintf("order [%s] already accepted", order.Id)
			s.logger.Warn(errorMsg)
			http.Error(w, errorMsg, http.StatusBadRequest)
			return
		}

		order.UpdatedAt = time.Now().UTC()
		order.Accepted = true

		if err = s.db.ReplaceOrder(order); err != nil {
			s.logger.Errorf("error updating order [%s]", orderId)
			http.Error(w, "error updating order: "+orderId, http.StatusInternalServerError)
			return
		}

		renderJSON(w, map[string]string{
			"status":  "ok",
			"orderId": orderId,
		})
	}
}

func (s *Server) startConsumer() {
	ctx := context.Background()
	for {
		msg, err := s.consumer.Consume(ctx)
		if err != nil {
			s.logger.Error("failed to consume message: ", err)
			continue
		}

		var order *store.Order
		if err := json.Unmarshal(msg.Value, &order); err != nil {
			s.logger.Warn("failed to unmarshal order: ", err)
			continue
		}

		s.mu.Lock()
		s.orders = append(s.orders, order)
		s.mu.Unlock()

		s.logger.Infof("new order received: %+v", order)
	}
}

func (s *Server) startOrderCleanupCron() {
	cleanupInterval := 10 * time.Minute
	cronInterval := 2 * time.Minute

	ticker := time.NewTicker(cronInterval)
	defer ticker.Stop()

	s.logger.Infof("starting order cleanup cron job. Will delete orders older than %v every %v", cleanupInterval, cronInterval)

	for {
		select {
		case <-ticker.C:
			s.cleanupOldOrders(cleanupInterval)
		case <-s.stopCron:
			s.logger.Info("stopping order cleanup cron job")
			return
		}
	}
}

func (s *Server) cleanupOldOrders(maxAge time.Duration) {
	cutoffTime := time.Now().Add(-maxAge)

	deletedCount, err := s.db.DeleteOldOrders(cutoffTime)
	if deletedCount == 0 {
		return
	}
	if err != nil {
		s.logger.Errorf("error cleaning up old orders: %v", err)
		return
	}

	if deletedCount > 0 {
		s.logger.Infof("cleaned up %d orders older than %v", deletedCount, cutoffTime)

		s.mu.Lock()
		filteredOrders := make([]*store.Order, 0)
		for _, order := range s.orders {
			if !order.CreatedAt.Before(cutoffTime) {
				filteredOrders = append(filteredOrders, order)
			}
		}
		s.orders = filteredOrders
		s.mu.Unlock()
	}
}

func (s *Server) Stop() {
	close(s.stopCron)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == s.configuration.LocalOriginUrl || origin == s.configuration.FrontOriginUrl {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) getWaiter(ctx context.Context) (*store.Waiter, error) {
	waiterId, err := s.getWaiterIdFromContext(ctx)
	if err != nil {
		return nil, err
	}

	waiter, err := s.db.GetWaiterById(waiterId)
	if err != nil {
		return nil, err
	}

	return waiter, nil
}

func renderJSON(w http.ResponseWriter, v interface{}) {
	js, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
