package otel

import (
	"go.opentelemetry.io/otel"

	"github.com/Alturino/ecommerce/internal/common/constants"
)

var Tracer = otel.Tracer(constants.MainEcommerce)
