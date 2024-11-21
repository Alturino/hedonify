package request

import "github.com/google/uuid"

type InsertCartRequest struct {
	UserID string `validate:"required,uuid" json:"userId"`
	Price  string `validate:"required"      json:"price"`
}

type CartItemRequest struct {
	CartId    string `validate:"required,uuid"  json:"cartId"`
	ProductId string `validate:"required,uuid"  json:"productId"`
	Price     string `validate:"required"       json:"price"`
	Quantity  int    `validate:"required,gte=1" json:"quantity"`
}

type FindCartByIdRequest struct {
	ID uuid.UUID `validate:"required, uuid"`
}
