package cmd

import (
	"context"

	"github.com/Alturino/ecommerce/cart/cmd"
)

func runCartService(c context.Context) {
	cmd.RunCartService(c)
}
