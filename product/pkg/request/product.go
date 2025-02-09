package request

import (
	"github.com/shopspring/decimal"
)

type Product struct {
	Name     string          `validate:"required" json:"name"`
	Price    decimal.Decimal `validate:"required" json:"price"`
	Quantity int             `validate:"required" json:"quantity"`
}

type FindProduct struct {
	Name     string
	MinPrice *decimal.Decimal `validate:"numeric"`
	MaxPrice *decimal.Decimal `validate:"numeric"`
}
