package error

import "errors"

var (
	ErrPasswordMismatch = errors.New("ErrPasswordMismatch")
	ErrUserNotFound     = errors.New("ErrUserNotFound")
)
