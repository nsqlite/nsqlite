package main

import (
	"context"
	"log"
	"os"

	"github.com/nsqlite/nsqlite/cmd/nsqlited/config"
)

func main() {
	cfg := config.MustParse(os.Args)
	ctx := context.Background()
	if err := run(ctx, cfg); err != nil {
		log.Fatal(err)
	}
}
