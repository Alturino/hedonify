package request

import "github.com/google/uuid"

type InsertOrderRequest struct {
	OrderItems []OrderItemRequest `validate:"required"      json:"orderItems"`
	CartId     uuid.UUID          `validate:"required,uuid" json:"cartId"`
	UserId     uuid.UUID          `validate:"required,uuid" json:"userId"`
}

type OrderItemRequest struct {
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
	UserId  string
	OrderId string
}
