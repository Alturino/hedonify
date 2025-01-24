package repository

import (
	"github.com/shopspring/decimal"

	"github.com/Alturino/ecommerce/product/response"
)

func (p Product) Response() response.Product {
	return response.Product{
		ID:        p.ID,
		Name:      p.Name,
		Price:     decimal.NewFromBigInt(p.Price.Int, p.Price.Exp),
		Quantity:  p.Quantity,
		CreatedAt: p.CreatedAt.Time,
		UpdatedAt: p.UpdatedAt.Time,
	}
}
