package store

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	Id             uuid.UUID `json:"id"`
	RestaurantName string    `json:"restaurant_name"`
	TableNumber    int       `json:"table_number"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Accepted       bool      `json:"accepted"`
}
