package cmd

import (
	"context"

	"github.com/Alturino/ecommerce/notification/cmd"
)

func runNotificationService(c context.Context) {
	cmd.RunNotificationService(c)
}
