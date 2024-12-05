package response

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/rs/zerolog"

	"github.com/Alturino/ecommerce/internal/otel/trace"
)

func WriteJsonResponse(
	c context.Context,
	w http.ResponseWriter,
	header map[string]string,
	body map[string]interface{},
) {
	c, span := trace.Tracer.Start(c, "WriteJsonResponse")
	defer span.End()

	logger := zerolog.Ctx(c)

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
		logger.Error().
			Err(err).
			Msgf("failed encode response body with error=%s", err.Error())
		return
	}
}
