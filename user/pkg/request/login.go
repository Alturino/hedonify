package request

import (
	"encoding/json"

	"github.com/rs/zerolog"
)

type LoginRequest struct {
	Email    string `validate:"required,email" json:"email"`
	Password string `validate:"required"       json:"password"`
}

func (l LoginRequest) MarshalZerologObject(e *zerolog.Event) {
	e.Str("email", l.Email).Str("password", "***")
}

func (l LoginRequest) MarshalJSON() ([]byte, error) {
	l.Password = "***"
	type L LoginRequest
	return json.Marshal(L(l))
}
