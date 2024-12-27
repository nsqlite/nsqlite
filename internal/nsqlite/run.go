package nsqlite

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nsqlite/nsqlite/internal/nsqlite/config"
	"github.com/nsqlite/nsqlite/internal/version"
)

// Run runs the NSQLite server.
func Run(ctx context.Context) error {
	conf := config.MustParse(os.Args)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println(version.ClientVersion())
	fmt.Println("Connecting to", conf.ParsedConnectionString.String())

	<-ctx.Done()
	fmt.Printf("\nGoodbye!\n\n")
	return nil
}
