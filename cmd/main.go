package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var version string

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	root := newRootCmd()
	if err := root.Execute(); err != nil {
		os.Exit(-1)
	}
}
