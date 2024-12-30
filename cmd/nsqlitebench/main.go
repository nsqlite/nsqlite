package main

import (
	"context"
	"log"

	"github.com/nsqlite/nsqlite/internal/nsqlitebench"
)

func main() {
	if err := nsqlitebench.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
