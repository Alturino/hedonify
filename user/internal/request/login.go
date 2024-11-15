package request

type LoginRequest struct {
	Email    string `validate:"required,email" json:"email"`
	Password string `validate:"required"       json:"password"`
}
