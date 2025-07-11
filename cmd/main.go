// package main implements the wifire cli.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

var version string

func Main() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	return newRootCmd().ExecuteContext(ctx)
}

func main() {
	if err := Main(); err != nil {
		os.Exit(1)
	}
}
