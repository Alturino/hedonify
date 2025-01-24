package repository

import (
	"encoding/json"

	"github.com/Alturino/ecommerce/order/internal/response"
)

func (f FindOrderByIdRow) ResponseOrder() (response.Order, error) {
	orderItems := []response.OrderItem{}
	err := json.Unmarshal(f.OrderItems, &orderItems)
	if err != nil {
		return response.Order{}, err
	}
	return response.Order{
		ID:         f.ID,
		UserId:     f.UserID,
		Status:     string(f.Status),
		OrderItems: orderItems,
		CreatedAt:  f.CreatedAt.Time,
		UpdatedAt:  f.UpdatedAt.Time,
	}, nil
}
