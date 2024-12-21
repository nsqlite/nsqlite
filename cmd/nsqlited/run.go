package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/nsqlite/nsqlite/cmd/nsqlited/config"
	"github.com/nsqlite/nsqlite/internal/log"
)

func run(ctx context.Context, cfg config.Config) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, os.Kill)
	defer cancel()

	logger := log.NewLogger(os.Stdout)
	logger.Info("Starting nsqlited", log.KV{
		"config": cfg,
	})

	return nil
}
