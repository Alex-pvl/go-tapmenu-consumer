package store

import (
	"context"
	"github.com/sirupsen/logrus"
	"time"

	"github.com/alex-pvl/go-tapmenu-consumer/internal/app/config"
	"github.com/google/uuid"
	"github.com/tarantool/go-tarantool/v2"
	"github.com/tarantool/go-tarantool/v2/datetime"
)

type Store struct {
	config *config.Configuration
	conn   *tarantool.Connection
	logger *logrus.Logger
}

func New(config *config.Configuration, logger *logrus.Logger) *Store {
	s := &Store{config: config, logger: logger}
	if err := s.connect(); err != nil {
		logger.Error(err)
	}
	return s
}

func (s *Store) connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.config.Timeout)*time.Second)
	defer cancel()

	dialer := tarantool.NetDialer{
		Address:  s.config.TarantoolAddress,
		User:     s.config.Username,
		Password: s.config.Password,
	}
	opts := tarantool.Opts{
		Timeout: time.Duration(s.config.Timeout) * time.Second,
	}

	conn, err := tarantool.Connect(ctx, dialer, opts)
	if err != nil {
		return err
	}

	s.conn = conn
	return nil
}

func (s *Store) GetOrder(id uuid.UUID) (*Order, error) {
	selectRequest := tarantool.NewSelectRequest(s.config.OrdersSpaceId).Key([]interface{}{id.String()})
	resp, err := s.conn.Do(selectRequest).Get()
	if err != nil {
		return nil, err
	}
	return mapToOrder(resp)
}

func (s *Store) GetOrderSlice() ([]*Order, error) {
	selectRequest := tarantool.NewSelectRequest(s.config.OrdersSpaceId)
	resp, err := s.conn.Do(selectRequest).Get()
	if err != nil {
		return nil, err
	}
	return mapToOrderSlice(resp, "")
}

func (s *Store) GetOrderSliceByRestaurant(restName string) ([]*Order, error) {
	selectRequest := tarantool.NewSelectRequest(s.config.OrdersSpaceId)
	resp, err := s.conn.Do(selectRequest).Get()
	if err != nil {
		return nil, err
	}
	return mapToOrderSlice(resp, restName)
}

func (s *Store) ReplaceOrder(order *Order) error {
	createdAt, _ := datetime.MakeDatetime(order.CreatedAt)
	updatedAt, _ := datetime.MakeDatetime(order.UpdatedAt)

	replaceRequest := tarantool.NewReplaceRequest(s.config.OrdersSpaceId).Tuple([]interface{}{
		order.Id.String(),
		order.RestaurantName,
		order.TableNumber,
		createdAt,
		updatedAt,
		order.Accepted,
	})
	_, err := s.conn.Do(replaceRequest).Get()
	return err
}

func (s *Store) DeleteOrder(id uuid.UUID) error {
	deleteRequest := tarantool.NewDeleteRequest(s.config.OrdersSpaceId).Key([]interface{}{id.String()})
	_, err := s.conn.Do(deleteRequest).Get()
	return err
}

func (s *Store) DeleteOldOrders(olderThan time.Time) (int, error) {
	orders, err := s.GetOrderSlice()
	if err != nil {
		return 0, err
	}

	deletedCount := 0
	for _, order := range orders {
		if order.CreatedAt.Before(olderThan) {
			if err := s.DeleteOrder(order.Id); err != nil {
				return deletedCount, err
			}
			deletedCount++
		}
	}

	return deletedCount, nil
}

func (s *Store) GetWaiterById(waiterId string) (*Waiter, error) {
	selectRequest := tarantool.NewSelectRequest(s.config.WaitersSpaceId).Key([]interface{}{waiterId})
	resp, err := s.conn.Do(selectRequest).Get()
	if err != nil {
		return nil, err
	}
	return mapToWaiter(resp)
}

func (s *Store) GetWaiterByUsername(username string) (*Waiter, error) {
	selectRequest := tarantool.NewSelectRequest(
		s.config.WaitersSpaceId,
	).Index(s.config.WaiterUsernameIdx).Key([]interface{}{username})
	resp, err := s.conn.Do(selectRequest).Get()
	if err != nil {
		return nil, err
	}
	return mapToWaiter(resp)
}
