package request

import "github.com/google/uuid"

type InsertOrder struct {
	OrderItems []OrderItem `validate:"required"      json:"orderItems"`
	CartId     uuid.UUID   `validate:"required,uuid" json:"cartId"`
	UserId     uuid.UUID   `validate:"required,uuid" json:"userId"`
}

type OrderItem struct {
	Price     string    `validate:"required"       json:"price"`
	Quantity  int       `validate:"required,gte=1" json:"quantity"`
	ProductId uuid.UUID `validate:"required,uuid"  json:"productId"`
}

type FindOrderByUserId struct {
	UserId uuid.UUID `validate:"uuid"`
}

type FindOrderById struct {
	OrderId uuid.UUID `validate:"required,uuid"`
}

type FindOrders struct {
	UserId  uuid.UUID `validate:"required,uuid"`
	OrderId uuid.UUID `validate:"required,uuid"`
}
