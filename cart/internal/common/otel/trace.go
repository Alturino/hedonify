package otel

import (
	"go.opentelemetry.io/otel"

	"github.com/Alturino/ecommerce/internal/common"
)

var Tracer = otel.Tracer(common.AppCartService)
