package request

import "github.com/google/uuid"

type InsertProductRequest struct {
	Name   string `validate:"required" json:"name"`
	Price  string `validate:"required" json:"price"`
	Amount int    `validate:"required" json:"amount"`
}

type FindProductRequest struct {
	Name string
	ID   uuid.UUID
}
