package response

import (
	"github.com/Alturino/ecommerce/order/pkg/request"
)

func (c Cart) Order() request.CreateOrder {
	orderItems := make([]request.OrderItem, len(c.CartItems))
	for i, item := range c.CartItems {
		orderItems[i] = request.OrderItem{
			ID:        item.ID,
			Price:     item.Price,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}
	}
	return request.CreateOrder{
		OrderItems: orderItems,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
		ID:         c.ID,
		UserId:     c.UserID,
	}
}
