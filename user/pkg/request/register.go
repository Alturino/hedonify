package request

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Register struct {
	Username string `validate:"required"       json:"username"`
	Email    string `validate:"required,email" json:"email"`
	Password string `validate:"required"       json:"password"`
}

func (l Register) MarshalZerologObject(e *zerolog.Event) {
	e.Str("email", l.Email).Str("username", l.Username)
}

func (r Register) MarshalJSON() ([]byte, error) {
	r.Password = "***"
	type R Register
	return json.Marshal(R(r))
}

type FindUserById struct {
	ID uuid.UUID `validate:"required,uuid" json:"id"`
}
