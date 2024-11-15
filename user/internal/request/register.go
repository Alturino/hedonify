package request

type RegisterRequest struct {
	Username string `validate:"required,username"`
	Email    string `validate:"required,email"`
	Password string `validate:"required,string"`
}
