package request

import (
	"time"

	"github.com/google/uuid"

	"github.com/Alturino/ecommerce/cart/response"
)

type Cart struct {
	CartItems []CartItem `validate:"required"      json:"cartItems"`
	UserID    uuid.UUID  `validate:"required,uuid" json:"userId"`
}

type CartItem struct {
	Price     string    `validate:"required"       json:"price"`
	Quantity  int       `validate:"required,gte=1" json:"quantity"`
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

func (c Cart) Cart() response.Cart {
	cartId := uuid.New()
	cartItems := make([]response.CartItem, len(c.CartItems))
	for _, item := range c.CartItems {
		cartItems = append(cartItems, item.CartItem(cartId))
	}
	return response.Cart{
		ID:        cartId,
		UserID:    c.UserID,
		CartItems: cartItems,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (c CartItem) CartItem(cartId uuid.UUID) response.CartItem {
	return response.CartItem{
		ID:        uuid.New(),
		CartID:    cartId,
		ProductID: c.ProductId,
		Quantity:  c.Quantity,
		Price:     c.Price,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
