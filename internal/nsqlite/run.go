package nsqlite

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nsqlite/nsqlite/internal/nsqlite/client"
	"github.com/nsqlite/nsqlite/internal/nsqlite/config"
	"github.com/nsqlite/nsqlite/internal/nsqlite/repl"
	"github.com/nsqlite/nsqlite/internal/version"
)

// Run runs the NSQLite CLI.
func Run(ctx context.Context) error {
	conf := config.MustParse(os.Args)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println(version.ClientVersion())

	clientInst := client.NewClient(conf.ParsedConnectionString)
	rp := repl.NewRepl(ctx, stop, conf, clientInst)
	defer rp.Shutdown()
	go func() {
		if err := rp.Start(); err != nil {
			fmt.Println(err)
			stop()
		}
	}()

	<-ctx.Done()
	fmt.Printf("\nGoodbye!\n\n")
	return nil
}
