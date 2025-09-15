package store

import (
	"errors"
	"github.com/google/uuid"
	"github.com/tarantool/go-tarantool/v2/datetime"
	"sort"
)

var NoOrdersError = errors.New("no orders found")

func mapToOrder(dbResponse []interface{}) (*Order, error) {
	if len(dbResponse) == 0 {
		return nil, errors.New("order not found")
	}
	return mapFromInterface(dbResponse[0].([]interface{})), nil
}

func mapToOrderSlice(dbResponse []interface{}, restName string) ([]*Order, error) {
	slice := make([]*Order, 0, len(dbResponse))
	if len(dbResponse) == 0 {
		return nil, NoOrdersError
	}

	for _, line := range dbResponse {
		order := mapFromInterface(line.([]interface{}))
		if order.Accepted {
			continue
		}
		if restName != "" && order.RestaurantName != restName {
			continue
		}
		slice = append(slice, order)
	}

	if len(slice) == 0 {
		return nil, NoOrdersError
	}

	sort.Slice(slice, func(i, j int) bool {
		return slice[i].CreatedAt.Before(slice[j].CreatedAt)
	})

	return slice, nil
}

func mapFromInterface(row []interface{}) *Order {
	id, _ := row[0].(string)
	restName, _ := row[1].(string)
	number, _ := row[2].(int8)
	createdAtTnt, _ := row[3].(datetime.Datetime)
	updatedAtTnt, _ := row[4].(datetime.Datetime)
	accepted, _ := row[5].(bool)

	createdAt := createdAtTnt.ToTime()
	updatedAt := updatedAtTnt.ToTime()

	return &Order{
		Id:             uuid.MustParse(id),
		RestaurantName: restName,
		TableNumber:    int(number),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
		Accepted:       accepted,
	}
}

func mapToWaiter(dbResponse []interface{}) (*Waiter, error) {
	if len(dbResponse) == 0 {
		return nil, errors.New("waiter not found")
	}

	row := dbResponse[0].([]interface{})

	waiterId, _ := row[0].(string)
	username, _ := row[1].(string)
	hashedPassword, _ := row[2].(string)
	restaurantName := row[3].(string)

	return &Waiter{
		Id:             uuid.MustParse(waiterId),
		Username:       username,
		HashedPassword: hashedPassword,
		RestaurantName: restaurantName,
	}, nil
}
