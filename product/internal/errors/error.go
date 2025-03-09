package errors

import "errors"

var (
	ErrFailedInsertingProduct = errors.New("ErrFailedInsertingProduct")
	ErrProductAlreadyExist    = errors.New("product already exist")
)
