package request

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type CreateOrder struct {
	OrderItems []OrderItem `validate:"required,gt=0" json:"order_items"`
	CreatedAt  time.Time   `validate:"required"      json:"created_at"`
	UpdatedAt  time.Time   `validate:"required"      json:"updated_at"`
	ID         uuid.UUID   `validate:"required,uuid" json:"id"`
	UserId     uuid.UUID   `validate:"required,uuid" json:"user_id"`
}

type OrderItem struct {
	CreatedAt time.Time       `validate:"required"       json:"created_at"`
	UpdatedAt time.Time       `validate:"required"       json:"updated_at"`
	ID        uuid.UUID       `validate:"required,uuid"  json:"id"`
	Price     decimal.Decimal `validate:"required"       json:"price"`
	ProductID uuid.UUID       `validate:"required,uuid"  json:"product_id"`
	Quantity  int32           `validate:"required,gte=1" json:"quantity"`
}
