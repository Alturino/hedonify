package http

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/common/errors"
	"github.com/Alturino/ecommerce/internal/common/otel"
)

func WriteJsonResponse(
	c context.Context,
	w http.ResponseWriter,
	header map[string]string,
	body map[string]interface{},
) {
	c, span := otel.Tracer.Start(c, "WriteJsonResponse")
	defer span.End()

	logger := zerolog.Ctx(c).With().Str("tag", "WriteJsonResponse").Logger()

	w.Header().Add(HeaderContentType, HeaderValueJson)

	var wg sync.WaitGroup
	for k, v := range header {
		wg.Add(1)
		go func() {
			w.Header().Add(k, v)
			wg.Done()
		}()
	}
	wg.Wait()

	if v, ok := body["statusCode"]; ok {
		w.WriteHeader(v.(int))
	}

	err := json.NewEncoder(w).Encode(body)
	if err != nil {
		errors.HandleError(err, logger, span)
		return
	}
}