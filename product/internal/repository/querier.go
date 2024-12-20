// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0

package repository

import (
	"context"

	"github.com/google/uuid"
)

type Querier interface {
	FindProductById(ctx context.Context, id uuid.UUID) (Product, error)
	FindProducts(ctx context.Context, arg FindProductsParams) (Product, error)
	GetProducts(ctx context.Context) ([]Product, error)
	InsertProduct(ctx context.Context, arg InsertProductParams) (Product, error)
}

var _ Querier = (*Queries)(nil)
