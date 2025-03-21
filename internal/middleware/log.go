package middleware

import (
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
		reqId := r.Header.Get(inHttp.KEY_HEADER_REQUEST_ID)
		if reqId == "" {
			reqId = uuid.NewString()
		}
		c, span := otel.Tracer.Start(
			r.Context(),
			"main Logging",
			trace.WithAttributes(
				attribute.String(constants.KEY_REQUEST_ID, reqId),
				attribute.String(constants.KEY_REQUEST_HOST, r.Host),
				attribute.String(constants.KEY_REQUEST_IP, r.RemoteAddr),
				attribute.String(constants.KEY_REQUEST_METHOD, r.Method),
				attribute.String(constants.KEY_REQUEST_URI, r.RequestURI),
				attribute.String(constants.KEY_REQUEST_URL, r.URL.String()),
			),
		)
		defer span.End()

		c = log.AttachRequestIDToContext(c, reqId)
		logger := zerolog.Ctx(c).
			With().
			Ctx(c).
			Str(constants.KEY_PROCESS, "attach request to logger").
			Dict(constants.KEY_REQUEST, zerolog.Dict().
				Any(constants.KEY_HEADER, r.Header).
				Str(constants.KEY_REQUEST_HOST, r.Host).
				Str(constants.KEY_REQUEST_IP, r.RemoteAddr).
				Str(constants.KEY_REQUEST_METHOD, r.Method).
				Str(constants.KEY_REQUEST_URI, r.RequestURI).
				Str(constants.KEY_REQUEST_URL, r.URL.String())).
			Str(constants.KEY_TAG, "middleware Logging").
			Logger().
			Hook(log.AttachTraceIdFromContext())
		logger.Debug().Msg("attached request value to logger")

		logger.Trace().Msg("attaching request value to context")
		c = logger.WithContext(c)
		r = r.WithContext(c)
		logger.Debug().Msg("attached request value to context")

		logger.Trace().Msg("next handler")
		next.ServeHTTP(w, r)
	})
}
