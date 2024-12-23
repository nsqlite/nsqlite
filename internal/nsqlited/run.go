package nsqlited

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nsqlite/nsqlite/internal/log"
	"github.com/nsqlite/nsqlite/internal/nsqlited/config"
	"github.com/nsqlite/nsqlite/internal/nsqlited/db"
	"github.com/nsqlite/nsqlite/internal/nsqlited/server"
)

// Run runs the NSQLite server.
func Run(ctx context.Context) error {
	conf := config.MustParse(os.Args)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := log.NewLogger(os.Stdout)
	logger.Info("starting NSQLite server")

	dbInstance, err := db.NewDB(db.Config{
		Logger:                 logger,
		Directory:              conf.DataDirectory,
		DisableOptimizations:   conf.DisableOptimizations,
		TransactionIdleTimeout: conf.TransactionIdleTimeout,
	})
	if err != nil {
		return fmt.Errorf("error starting database: %w", err)
	}
	defer func() {
		if err := dbInstance.Close(); err != nil {
			logger.Error("error closing database:", log.KV{"error": err})
		}
	}()

	serv, err := server.NewServer(server.Config{
		Logger:     logger,
		Db:         dbInstance,
		ListenHost: conf.ListenHost,
		ListenPort: conf.ListenPort,
	})
	if err != nil {
		return fmt.Errorf("error creating server: %w", err)
	}
	defer func() {
		if err := serv.Stop(); err != nil {
			logger.Error("error stopping server:", log.KV{"error": err})
		}
	}()
	go func() {
		if err := serv.Start(); err != nil {
			logger.Error("server stopped with error:", log.KV{"error": err})
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("goodbye! gracefully shutting down NSQLite server")
	return nil
}
