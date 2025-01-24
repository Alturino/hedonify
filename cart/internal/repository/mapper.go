package repository

import (
	"encoding/json"

	"github.com/Alturino/ecommerce/cart/response"
)

func (f FindCartByIdRow) Response() (response.Cart, error) {
	cartItems := []response.CartItem{}
	err := json.Unmarshal(f.CartItems, &cartItems)
	if err != nil {
		return response.Cart{}, err
	}
	return response.Cart{
		ID:        f.ID,
		UserID:    f.UserID,
		CartItems: cartItems,
		CreatedAt: f.CreatedAt.Time,
		UpdatedAt: f.UpdatedAt.Time,
	}, nil
}
