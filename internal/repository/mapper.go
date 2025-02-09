package repository

import (
	"encoding/json"

	"github.com/shopspring/decimal"

	cartResponse "github.com/Alturino/ecommerce/cart/pkg/response"
	orderResponse "github.com/Alturino/ecommerce/order/pkg/response"
	productResponse "github.com/Alturino/ecommerce/product/pkg/response"
)

func (p Product) Response() productResponse.Product {
	return productResponse.Product{
		ID:        p.ID,
		Name:      p.Name,
		Price:     decimal.NewFromBigInt(p.Price.Int, p.Price.Exp),
		Quantity:  p.Quantity,
		CreatedAt: p.CreatedAt.Time,
		UpdatedAt: p.UpdatedAt.Time,
	}
}

func (o GetOrdersRow) Response() (orderResponse.Order, error) {
	orderItems := []orderResponse.OrderItem{}
	err := json.Unmarshal(o.OrderItems, &orderItems)
	if err != nil {
		return orderResponse.Order{}, err
	}
	return orderResponse.Order{
		CreatedAt:  o.CreatedAt.Time,
		UpdatedAt:  o.UpdatedAt.Time,
		OrderItems: orderItems,
		Status:     string(o.Status),
		ID:         o.ID,
		UserId:     o.UserID,
	}, nil
}

func (f FindCartByIdRow) Response() (cartResponse.Cart, error) {
	cartItems := []cartResponse.CartItem{}
	err := json.Unmarshal(f.CartItems, &cartItems)
	if err != nil {
		return cartResponse.Cart{}, err
	}
	return cartResponse.Cart{
		ID:        f.ID,
		UserID:    f.UserID,
		CartItems: cartItems,
		CreatedAt: f.CreatedAt.Time,
		UpdatedAt: f.UpdatedAt.Time,
	}, nil
}

func (f FindOrderByIdRow) ResponseOrder() (orderResponse.Order, error) {
	orderItems := []orderResponse.OrderItem{}
	err := json.Unmarshal(f.OrderItems, &orderItems)
	if err != nil {
		return orderResponse.Order{}, err
	}
	return orderResponse.Order{
		ID:         f.ID,
		UserId:     f.UserID,
		Status:     string(f.Status),
		OrderItems: orderItems,
		CreatedAt:  f.CreatedAt.Time,
		UpdatedAt:  f.UpdatedAt.Time,
	}, nil
}
