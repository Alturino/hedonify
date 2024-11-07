package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo, AddSource: true})).
		With(
			slog.Int("pid", os.Getpid()),
			slog.Int("gid", os.Getgid()),
			slog.Int("uid", os.Getuid()),
		)
	slog.SetDefault(logger)

	c := context.Background()
	c, stop := signal.NotifyContext(c, os.Interrupt, os.Kill, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		logger.InfoContext(c, "shutting down")
		stop()
		logger.InfoContext(c, "shutdown server")
	}()
}
