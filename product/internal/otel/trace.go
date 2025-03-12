package otel

import (
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/Alturino/ecommerce/internal/constants"
)

var Tracer = otel.Tracer(
	constants.APP_PRODUCT_SERVICE,
	trace.WithInstrumentationAttributes(semconv.ServiceName(constants.APP_PRODUCT_SERVICE)),
)
