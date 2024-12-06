package request

import "github.com/google/uuid"

type InsertCart struct {
	CartItems []CartItem `validate:"required"      json:"cartItems"`
	UserID    uuid.UUID  `validate:"required,uuid" json:"userId"`
}

type CartItem struct {
	Price     string    `validate:"required"       json:"price"`
	Quantity  int       `validate:"required,gte=1" json:"quantity"`
	ProductId uuid.UUID `validate:"required,uuid"  json:"productId"`
}

type FindCartById struct {
	ID uuid.UUID `validate:"required, uuid"`
}

type RemoveCartItem struct {
	ID     uuid.UUID `validate:"required, uuid"`
	CartId uuid.UUID `validate:"required, uuid"`
}

type InsertCartItem struct {
	CartId    uuid.UUID `validate:"required, uuid"`
	ProductId uuid.UUID `validate:"required,uuid"  json:"productId"`
	Price     string    `validate:"required"       json:"price"`
	Quantity  int       `validate:"required,gte=1" json:"quantity"`
}
