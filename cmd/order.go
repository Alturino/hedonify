package cmd

import (
	"context"

	"github.com/Alturino/ecommerce/order/cmd"
)

func runOrderService(c context.Context) {
	cmd.RunOrderService(c)
}
