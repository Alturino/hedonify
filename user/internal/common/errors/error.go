package error

import "errors"

var (
	ErrPasswordMismatch = errors.New("password mismatch")
	ErrUserNotFound     = errors.New("user not found")
	ErrEmailExist       = errors.New("email already exist")
)
