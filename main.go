package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Alturino/ecommerce/internal/log"
)

func main() {
	logger := log.InitLogger("/var/log/ecommerce.log")

	c := context.Background()
	logger.Info().Msg("start")
	c, stop := signal.NotifyContext(c, os.Interrupt, os.Kill, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger.Info().Msg("end")
}
