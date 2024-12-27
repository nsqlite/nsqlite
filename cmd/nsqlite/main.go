package main

import (
	"context"
	"log"

	"github.com/nsqlite/nsqlite/internal/nsqlite"
)

func main() {
	if err := nsqlite.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
