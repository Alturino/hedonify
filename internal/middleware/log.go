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

	"github.com/Alturino/ecommerce/internal/constants"
	inHttp "github.com/Alturino/ecommerce/internal/http"
	"github.com/Alturino/ecommerce/internal/log"
	"github.com/Alturino/ecommerce/internal/otel"
)

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(inHttp.KEY_HEADER_REQUEST_ID)
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c, span := otel.Tracer.Start(
			r.Context(),
			"main Logging",
			trace.WithAttributes(
				attribute.String(constants.KEY_REQUEST_ID, requestID),
				attribute.String(constants.KEY_REQUEST_HOST, r.Host),
				attribute.String(constants.KEY_REQUEST_IP, r.RemoteAddr),
				attribute.String(constants.KEY_REQUEST_METHOD, r.Method),
				attribute.String(constants.KEY_REQUEST_URI, r.RequestURI),
				attribute.String(constants.KEY_REQUEST_URL, r.URL.String()),
			),
		)
		defer span.End()

		var buffer bytes.Buffer
		tee := io.TeeReader(r.Body, &buffer)
		requestBody := map[string]interface{}{}
		json.NewDecoder(tee).Decode(&requestBody)
		if requestBody["password"] != nil {
			requestBody["password"] = "****"
		}
		r.Body = io.NopCloser(&buffer)

		logger := zerolog.Ctx(c).
			With().
			Str(constants.KEY_REQUEST_ID, requestID).
			Dict(constants.KEY_REQUEST, zerolog.Dict().
				Any(constants.KEY_HEADER, r.Header).
				Str(constants.KEY_REQUEST_HOST, r.Host).
				Str(constants.KEY_REQUEST_IP, r.RemoteAddr).
				Str(constants.KEY_REQUEST_METHOD, r.Method).
				Str(constants.KEY_REQUEST_URI, r.RequestURI).
				Str(constants.KEY_REQUEST_URL, r.URL.String()).
				Any(constants.KEY_BODY, requestBody)).
			Str(constants.KEY_TAG, "Logging").Logger()

		logger.Trace().Msg("attached request value to logger")

		logger.Trace().Msg("attaching request value to context")
		c = log.AttachRequestIDToContext(c, requestID)
		c = logger.WithContext(c)
		r = r.WithContext(c)
		logger.Trace().Msg("attached request value to context")

		logger.Trace().Msg("next handler")
		next.ServeHTTP(w, r)
	})
}
