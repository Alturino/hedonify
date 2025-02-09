package request

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Cart struct {
	CartItems []CartItem `validate:"required" json:"cart_items"`
}

type CartItem struct {
	ProductId uuid.UUID       `validate:"required,uuid"  json:"product_id"`
	Price     decimal.Decimal `validate:"required"       json:"price"`
	Quantity  int32           `validate:"required,gte=1" json:"quantity"`
}

type RemoveCart struct {
	ID     uuid.UUID `validate:"required, uuid" json:"id"`
	UserId uuid.UUID `validate:"required, uuid" json:"user_id"`
}

type RemoveCartItem struct {
	ID     uuid.UUID `validate:"required, uuid"`
	CartId uuid.UUID `validate:"required, uuid"`
	UserId uuid.UUID `validate:"required, uuid"`
}

type InsertCartItem struct {
	CartId    uuid.UUID       `validate:"required,uuid"  json:"cartId"`
	ProductId uuid.UUID       `validate:"required,uuid"  json:"productId"`
	Price     decimal.Decimal `validate:"required"       json:"price"`
	Quantity  int             `validate:"required,gte=1" json:"quantity"`
}

type CheckoutCart struct {
	UserId uuid.UUID `validate:"required,uuid" json:"userId"`
	CartId uuid.UUID `validate:"required,uuid" json:"cartId"`
}

type FindCartById struct {
	ID     uuid.UUID `validate:"required, uuid" json:"id"`
	UserId uuid.UUID `validate:"required, uuid" json:"userId"`
}
