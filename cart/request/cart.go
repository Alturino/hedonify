package request

import "github.com/google/uuid"

type InsertCartRequest struct {
	CartItems []CartItemRequest `validate:"required"      json:"cartItems"`
	UserID    uuid.UUID         `validate:"required,uuid" json:"userId"`
}

type CartItemRequest struct {
	Price     string    `validate:"required"       json:"price"`
	Quantity  int       `validate:"required,gte=1" json:"quantity"`
	ProductId uuid.UUID `validate:"required,uuid"  json:"productId"`
}

type FindCartByIdRequest struct {
	ID uuid.UUID `validate:"required, uuid"`
}
