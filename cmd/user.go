package cmd

import (
	"context"

	"github.com/Alturino/ecommerce/user/cmd"
)

func runUserService(c context.Context) {
	cmd.RunUserService(c)
}
