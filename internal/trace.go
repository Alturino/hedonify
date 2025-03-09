package internal

import (
	"go.opentelemetry.io/otel"

	"github.com/Alturino/ecommerce/internal/constants"
)

var Tracer = otel.Tracer(constants.APP_MAIN_ECOMMERCE)
