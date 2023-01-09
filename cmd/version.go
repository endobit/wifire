package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("version - %s", cmd.Root().Version)
		},
	}
}
