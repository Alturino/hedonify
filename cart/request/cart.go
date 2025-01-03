package request

import (
	"github.com/google/uuid"
)

type Cart struct {
	CartItems []CartItem `validate:"required" json:"cartItems"`
}

type CartItem struct {
	Price     string    `validate:"required"       json:"price"`
	Quantity  int32     `validate:"required,gte=1" json:"quantity"`
	ProductId uuid.UUID `validate:"required,uuid"  json:"productId"`
}

type RemoveCartItem struct {
	ID     uuid.UUID `validate:"required, uuid"`
	CartId uuid.UUID `validate:"required, uuid"`
}

type InsertCartItem struct {
	CartId    uuid.UUID `validate:"required,uuid"  json:"cartId"`
	ProductId uuid.UUID `validate:"required,uuid"  json:"productId"`
	Price     string    `validate:"required"       json:"price"`
	Quantity  int       `validate:"required,gte=1" json:"quantity"`
}
