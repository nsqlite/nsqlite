package nsqlite

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nsqlite/nsqlite/internal/nsqlite/config"
	"github.com/nsqlite/nsqlite/internal/nsqlite/repl"
	"github.com/nsqlite/nsqlite/internal/version"
	"github.com/nsqlite/nsqlitego/nsqlitehttp"
)

// Run runs the NSQLite CLI.
func Run(ctx context.Context) error {
	conf := config.MustParse(os.Args)

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println(version.ClientVersion())

	client, err := nsqlitehttp.NewClient(conf.ConnectionString)
	if err != nil {
		return err
	}

	rp := repl.NewRepl(ctx, stop, conf, client)
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
