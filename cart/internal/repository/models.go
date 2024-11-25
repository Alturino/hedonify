// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0

package repository

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Cart struct {
	ID        uuid.UUID        `json:"id"`
	UserID    uuid.UUID        `json:"user_id"`
	CreatedAt pgtype.Timestamp `json:"created_at"`
	UpdatedAt pgtype.Timestamp `json:"updated_at"`
}

type CartItem struct {
	ID        uuid.UUID        `json:"id"`
	CartID    uuid.UUID        `json:"cart_id"`
	ProductID uuid.UUID        `json:"product_id"`
	Quantity  int32            `json:"quantity"`
	Price     pgtype.Numeric   `json:"price"`
	CreatedAt pgtype.Timestamp `json:"created_at"`
	UpdatedAt pgtype.Timestamp `json:"updated_at"`
}
