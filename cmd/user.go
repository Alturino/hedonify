package cmd

import (
	"context"

	"github.com/Alturino/ecommerce/user/cmd"
)

func startUserService(c context.Context) {
	cmd.RunUserService(c)
}
