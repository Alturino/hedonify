package errors

import (
	"errors"
)

var (
	ErrEmptyAuth       = errors.New("missing authorization")
	ErrEmptySubject    = errors.New("missing subject")
	ErrTokenInvalid    = errors.New("invalid token")
	ErrFailedHashToken = errors.New("failed hashing token")
	ErrOutOfStock      = errors.New("product is out of stock")
)
