package errors

import (
	"errors"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrEmptyAuth       = errors.New("missing authorization")
	ErrEmptySubject    = errors.New("missing subject")
	ErrTokenInvalid    = errors.New("invalid token")
	ErrFailedHashToken = errors.New("failed hashing token")
)

func HandleError(err error, logger zerolog.Logger, span trace.Span) {
	if err == nil {
		return
	}
	logger.Error().Err(err).Msg(err.Error())
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)
}
