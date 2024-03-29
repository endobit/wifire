// package main implements the wifire cli.
package main

import (
	"os"

	"log/slog"

	"github.com/endobit/clog"
)

var version string

func main() {
	slog.SetDefault(slog.New(clog.NewHandler(os.Stderr)))

	if err := newRootCmd().Execute(); err != nil {
		os.Exit(-1)
	}
}
