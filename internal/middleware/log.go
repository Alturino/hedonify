package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
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

type responseWriterWrapper struct {
	w          http.ResponseWriter
	body       bytes.Buffer
	statusCode int
}

func (rww *responseWriterWrapper) WriteHeader(statusCode int) {
	rww.w.WriteHeader(statusCode)
	rww.statusCode = statusCode
}

func (rww *responseWriterWrapper) Write(b []byte) (int, error) {
	rww.body.Write(b)
	return rww.w.Write(b)
}

func (rww *responseWriterWrapper) Header() http.Header {
	return rww.w.Header()
}

func (rww *responseWriterWrapper) Run(e *zerolog.Event, level zerolog.Level, message string) {
	if rww.body.Len() < 0 {
		err := errors.New("response body is empty")
		e.Err(err).Msg(err.Error())
		return
	}
	response := map[string]interface{}{}
	err := json.NewDecoder(io.NopCloser(&rww.body)).Decode(&response)
	if err != nil {
		e.Err(err).Msg(err.Error())
		return
	}
	e.Dict(
		constants.KEY_RESPONSE,
		zerolog.Dict().
			Any(constants.KEY_BODY, response).
			Any(constants.KEY_HEADER, rww.w.Header()),
	)
}

func newResponseWriterWrapper(w http.ResponseWriter) *responseWriterWrapper {
	var buffer bytes.Buffer
	return &responseWriterWrapper{w: w, body: buffer, statusCode: http.StatusOK}
}

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

		responseWriterWrapper := newResponseWriterWrapper(w)
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
			Str(constants.KEY_TAG, "Logging").Logger().
			Hook(responseWriterWrapper)

		logger.Trace().Msg("attached request value to logger")

		logger.Trace().Msg("attaching request value to context")
		c = log.AttachRequestIDToContext(c, requestID)
		r = r.WithContext(logger.WithContext(c))
		logger.Trace().Msg("attached request value to context")

		logger.Trace().Msg("next handler")
		next.ServeHTTP(responseWriterWrapper, r)
	})
}
