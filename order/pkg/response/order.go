package response

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Order struct {
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	OrderItems []OrderItem `json:"order_items"`
	Status     string      `json:"status"`
	ID         uuid.UUID   `json:"id"`
	UserId     uuid.UUID   `json:"user_id"`
}

type OrderItem struct {
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	ID        uuid.UUID       `json:"id"`
	OrderId   uuid.UUID       `json:"order_id"`
	ProductId uuid.UUID       `json:"product_id"`
	Price     decimal.Decimal `json:"price"`
	Quantity  int32           `json:"quantity"`
}
