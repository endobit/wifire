package main

import (
	"bufio"
	"encoding/json"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/endobit/wifire"
)

func newPlotCmd() *cobra.Command {
	var (
		input   string
		output  string
		markers []time.Duration
	)

	cmd := cobra.Command{
		Use:   "plot",
		Short: "Create a scatter plot from a previous run",
		RunE: func(cmd *cobra.Command, args []string) error {
			fin, err := os.Open(input)
			if err != nil {
				return err
			}
			defer fin.Close()

			var temps []wifire.Status

			for s := bufio.NewScanner(fin); s.Scan(); {
				var status wifire.Status

				if err := json.Unmarshal(s.Bytes(), &status); err != nil {
					return err
				}

				temps = append(temps, status)
			}

			p := wifire.NewPlotter(wifire.PlotterOptions{
				Title:   temps[0].Time.Format(time.ANSIC),
				Data:    temps,
				Markers: markers,
			})

			plot, err := p.Plot()
			if err != nil {
				return err
			}

			if err := plot.Save(800, 300, output); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&input, "input", "i", "", "input file")
	cmd.Flags().StringVarP(&output, "output", "o", "wifire.png", "output file")
	cmd.Flags().DurationSliceVar(&markers, "marker", nil, "set a time marker (e.g. \"4h30m\") ")

	if err := cmd.MarkFlagRequired("input"); err != nil {
		panic(err)
	}

	return &cmd
}
