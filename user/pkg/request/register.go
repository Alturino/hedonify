package request

import "github.com/google/uuid"

type Register struct {
	Username string `validate:"required"       json:"username"`
	Email    string `validate:"required,email" json:"email"`
	Password string `validate:"required"       json:"password"`
}

type FindUserById struct {
	ID uuid.UUID `validate:"required,uuid" json:"id"`
}
