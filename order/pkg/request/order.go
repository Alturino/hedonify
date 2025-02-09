package request

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/trace"

	"github.com/Alturino/ecommerce/order/internal/response"
)

type CreateOrder struct {
	OrderItems    []OrderItem          `validate:"required,gt=0" json:"order_items"`
	CreatedAt     time.Time            `validate:"required"      json:"created_at"`
	UpdatedAt     time.Time            `validate:"required"      json:"updated_at"`
	ID            uuid.UUID            `validate:"required,uuid" json:"id"`
	UserId        uuid.UUID            `validate:"required,uuid" json:"user_id"`
	ResultChannel chan response.Result `                         json:"-"`
	TraceLink     trace.Link           `                         json:"-"`
}

type FindOrderByUserId struct {
	UserId uuid.UUID `validate:"uuid"`
}

type FindOrderById struct {
	UserId  uuid.UUID `validate:"required,uuid"`
	OrderId uuid.UUID `validate:"required,uuid"`
}

type FindOrders struct {
	UserId  uuid.UUID `validate:"required,uuid"`
	OrderId uuid.UUID `validate:"required,uuid"`
}

type OrderItem struct {
	CreatedAt time.Time       `validate:"required"       json:"created_at"`
	UpdatedAt time.Time       `validate:"required"       json:"updated_at"`
	ID        uuid.UUID       `validate:"required,uuid"  json:"id"`
	OrderID   uuid.UUID       `validate:"required,uuid"  json:"order_id"`
	ProductID uuid.UUID       `validate:"required,uuid"  json:"product_id"`
	Price     decimal.Decimal `validate:"required"       json:"price"`
	Quantity  int32           `validate:"required,gte=1" json:"quantity"`
}
