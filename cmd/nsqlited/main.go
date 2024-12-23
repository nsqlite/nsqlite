package main

import (
	"context"
	"log"

	"github.com/nsqlite/nsqlite/internal/nsqlited"
)

func main() {
	if err := nsqlited.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
