package otel

import (
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/Alturino/ecommerce/internal/constants"
)

var Tracer = otel.Tracer(
	constants.APP_ORDER_SERVICE,
	trace.WithInstrumentationAttributes(semconv.ServiceNameKey.String(constants.APP_ORDER_SERVICE)),
)
