package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	zl "github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/Alturino/ecommerce/internal/constants"
)

func Start() {
	logger := zl.Logger.
		With().
		Str(constants.KEY_APP_NAME, constants.APP_MAIN_ECOMMERCE).
		Str(constants.KEY_TAG, "main Start").
		Logger()

	logger.Trace().Msg("adding listener for SIGINT and SIGTERM")
	c, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger.Info().Msg("added listener for SIGINT and SIGTERM")

	rootCmd := &cobra.Command{}
	commands := []*cobra.Command{
		{
			Use:   "cart",
			Short: "Run cart service",
			Run: func(cmd *cobra.Command, args []string) {
				runCartService(cmd.Context())
			},
		},
		{
			Use:   "notification",
			Short: "Run notification service",
			Run: func(cmd *cobra.Command, args []string) {
				runNotificationService(cmd.Context())
			},
		},
		{
			Use:   "order",
			Short: "Run order service",
			Run: func(cmd *cobra.Command, args []string) {
				runOrderService(cmd.Context())
			},
		},
		{
			Use:   "product",
			Short: "Run product service",
			Run: func(cmd *cobra.Command, args []string) {
				runProductService(cmd.Context())
			},
		},
		{
			Use:   "user",
			Short: "Run user service",
			Run: func(cmd *cobra.Command, args []string) {
				runUserService(cmd.Context())
			},
		},
	}
	rootCmd.AddCommand(commands...)
	if err := rootCmd.ExecuteContext(c); err != nil {
		logger.Fatal().Err(err).Msgf("error when executing command=%s", err.Error())
	}
}
