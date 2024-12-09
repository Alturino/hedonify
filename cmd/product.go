package cmd

import (
	"context"

	"github.com/Alturino/ecommerce/product/cmd"
)

func runProductService(c context.Context) {
	cmd.RunProductService(c)
}
