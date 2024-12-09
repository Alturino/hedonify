package cmd

import (
	"context"

	"github.com/Alturino/ecommerce/shop/cmd"
)

func runShopService(c context.Context) {
	cmd.RunShopService(c)
}
