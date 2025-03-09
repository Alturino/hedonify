package errors

import (
	"errors"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrEmptyAuth       = errors.New("missing authorization")
	ErrEmptySubject    = errors.New("missing subject")
	ErrTokenInvalid    = errors.New("invalid token")
	ErrFailedHashToken = errors.New("failed hashing token")
	ErrOutOfStock      = errors.New("product is out of stock")
)

func HandleError(err error, span trace.Span) {
	if err == nil {
		return
	}
	span.AddEvent(err.Error())
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)
}
