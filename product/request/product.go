package request

import "github.com/google/uuid"

type InsertProductRequest struct {
	Name     string `validate:"required" json:"name"`
	Price    string `validate:"required" json:"price"`
	Quantity int    `validate:"required" json:"quantity"`
}

type FindProductRequest struct {
	Name string
	ID   uuid.UUID
}
