package cmd

import (
	"context"

	"github.com/Alturino/ecommerce/product/cmd"
)

func startProductService(c context.Context) {
	cmd.StartProductService(c)
}
