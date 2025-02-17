package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	commonHttp "github.com/Alturino/ecommerce/internal/common/http"
	"github.com/Alturino/ecommerce/internal/common/otel"
	"github.com/Alturino/ecommerce/internal/log"
)

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(commonHttp.KEY_HEADER_REQUEST_ID)
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c, span := otel.Tracer.Start(
			r.Context(),
			"main Logging",
			trace.WithAttributes(
				attribute.String(log.KEY_REQUEST_ID, requestID),
				attribute.String(log.KEY_REQUEST_HOST, r.Host),
				attribute.String(log.KEY_REQUEST_IP, r.RemoteAddr),
				attribute.String(log.KEY_REQUEST_METHOD, r.Method),
				attribute.String(log.KEY_REQUEST_URI, r.RequestURI),
				attribute.String(log.KEY_REQUEST_URL, r.URL.String()),
			),
		)
		defer span.End()

		var buffer bytes.Buffer
		teeReader := io.TeeReader(r.Body, &buffer)
		requestBody := map[string]interface{}{}
		json.NewDecoder(teeReader).Decode(&requestBody)
		r.Body = io.NopCloser(&buffer)

		logger := zerolog.Ctx(c).
			With().
			Str(log.KEY_REQUEST_ID, requestID).
			Dict(log.KEY_REQUEST, zerolog.Dict().
				Any(log.KEY_REQUEST_HEADER, r.Header).
				Str(log.KEY_REQUEST_HOST, r.Host).
				Str(log.KEY_REQUEST_IP, r.RemoteAddr).
				Str(log.KEY_REQUEST_METHOD, r.Method).
				Str(log.KEY_REQUEST_URI, r.RequestURI).
				Str(log.KEY_REQUEST_URL, r.URL.String()).
				Any(log.KEY_REQUEST_BODY, requestBody)).
			Str(log.KEY_TAG, "Logging").
			Logger()
		logger.Info().Msg("attached request value to logger")

		logger.Info().Msg("attaching request value to context")
		c = log.AttachRequestIDToContext(c, requestID)
		c = logger.WithContext(c)
		r = r.WithContext(c)
		logger.Info().Msg("attached request value to context")

		logger.Info().Msg("next handler")
		next.ServeHTTP(w, r)
	})
}
