package response

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Order struct {
	OrderItems []OrderItem `json:"orderItems"`
	ID         uuid.UUID   `json:"id"`
	UserId     uuid.UUID   `json:"userId"`
}

type OrderItem struct {
	ID        uuid.UUID       `json:"id"`
	OrderId   uuid.UUID       `json:"orderId"`
	ProductId uuid.UUID       `json:"productId"`
	Quantity  int32           `json:"quantity"`
	Price     decimal.Decimal `json:"price"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}
