package tapmenu

import (
	"context"
	"encoding/json"
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
) *Server {
	return &Server{
		configuration: configuration,
		logger:        logrus.New(),
		router:        mux.NewRouter(),
		db:            db,
		consumer:      consumer,
		orders:        make([]*store.Order, 0),
		stopCron:      make(chan struct{}),
	}
}

func (s *Server) Start() error {
	if err := s.configureLogger(); err != nil {
		return err
	}
	s.configureRouter()

	go s.startConsumer()
	go s.startOrderCleanupCron()

	s.logger.Info("starting server on port ", s.configuration.BindAddress)

	return http.ListenAndServe(s.configuration.BindAddress, s.router)
}

func (s *Server) configureLogger() error {
	level, err := logrus.ParseLevel(s.configuration.LogLevel)
	if err != nil {
		return err
	}

	s.logger.SetLevel(level)
	return nil
}

func (s *Server) configureRouter() {
	s.router.HandleFunc("/login", s.handleLogin()).Methods(http.MethodPost)
	s.router.HandleFunc("/orders", s.handleGetOrders()).Methods(http.MethodGet)
	s.router.HandleFunc("/orders/{orderId}/accept", s.handleAcceptOrder()).Methods(http.MethodPost)
}

func (s *Server) handleGetOrders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.authorize(r); err != nil {
			s.logger.Error(err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

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

		s.mu.RLock()
		defer s.mu.RUnlock()

		var err error
		s.orders, err = s.db.GetOrderSlice()
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
		if err := s.authorize(r); err != nil {
			s.logger.Error(err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		vars := mux.Vars(r)
		orderId := vars["orderId"]

		s.mu.Lock()
		defer s.mu.Unlock()

		found := false
		for i := range s.orders {
			orderIdParsed, err := uuid.Parse(orderId)
			if err != nil {
				s.logger.Errorf("error parsing order id %s", orderId)
				continue
			}
			if s.orders[i].Id == orderIdParsed {
				s.orders[i].Accepted = true
				found = true
				break
			}
		}

		if !found {
			http.Error(w, "order not found", http.StatusNotFound)
			return
		}

		order, err := s.db.GetOrder(uuid.MustParse(orderId))
		if err != nil {
			s.logger.Errorf("error getting order [%s]", orderId)
		}

		order.UpdatedAt = time.Now().UTC()
		order.Accepted = true

		if err = s.db.ReplaceOrder(order); err != nil {
			s.logger.Errorf("error updating order [%s]", orderId)
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

	s.logger.Infof("Starting order cleanup cron job. Will delete orders older than %v every %v", cleanupInterval, cronInterval)

	for {
		select {
		case <-ticker.C:
			s.cleanupOldOrders(cleanupInterval)
		case <-s.stopCron:
			s.logger.Info("Stopping order cleanup cron job")
			return
		}
	}
}

func (s *Server) cleanupOldOrders(maxAge time.Duration) {
	cutoffTime := time.Now().Add(-maxAge)

	deletedCount, err := s.db.DeleteOldOrders(cutoffTime)
	if err != nil || deletedCount == 0 {
		s.logger.Warnf("Error cleaning up old orders: %v", err)
		return
	}

	if deletedCount > 0 {
		s.logger.Infof("Cleaned up %d orders older than %v", deletedCount, cutoffTime)

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

func renderJSON(w http.ResponseWriter, v interface{}) {
	js, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
