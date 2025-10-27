package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"endobit.io/table"
	"endobit.io/wifire"
)

func newForecastCmd() *cobra.Command { //nolint:gocognit
	var (
		input      string
		actualTime string
	)

	cmd := cobra.Command{
		Use:   "forecast",
		Short: "Show ETA forecasts from historical data as if it were real-time",
		Long: `The forecast command reads a JSON log file and shows what the ETA predictions 
would have been at each point in time, using the exponential approach model. This allows you to
validate the accuracy of the predictions against the actual completion time.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			fin, err := os.Open(input)
			if err != nil {
				return err
			}
			defer fin.Close()

			// Parse actual finish time if provided
			var actualFinish *time.Time

			if actualTime != "" {
				parsed, err := time.Parse(time.RFC3339, actualTime)
				if err != nil {
					return fmt.Errorf("invalid actual time format (use RFC3339): %w", err)
				}

				actualFinish = &parsed
			}

			// Read all entries first
			var entries []wifire.Status

			scanner := bufio.NewScanner(fin)
			for scanner.Scan() {
				var status wifire.Status

				if err := json.Unmarshal(scanner.Bytes(), &status); err != nil {
					continue // Skip invalid entries
				}

				if status.Probe > 0 && status.ProbeSet > 0 {
					entries = append(entries, status)
				}
			}

			if err := scanner.Err(); err != nil {
				return err
			}

			if len(entries) == 0 {
				fmt.Println("No valid probe data found in input file")

				return nil
			}

			fmt.Printf("Forecasting ETA predictions from %d entries\n", len(entries))

			if actualFinish != nil {
				fmt.Printf("Actual finish time: %s\n", actualFinish.Format("15:04:05"))
			}

			fmt.Println()

			// Initialize exponential predictor
			ep := wifire.NewExponentialPredictor() //nolint:varnamelen

			type row struct {
				Time   string
				Delta  string `table:"\n(s)"`
				Grill  int
				Probe  int
				Target int
				// F or C depending on grill settings
				// TODO: are units in the status data?
				Velocity string `table:"\n(°/h)"`
				Filtered string
				ExpETA   string
				Actual   string `table:",omitempty"`
				Accuracy string `table:",omitempty"`
			}

			output := table.New()

			// Process each entry as if it were real-time
			for i := range entries {
				entry := &entries[i]

				// Calculate time delta from previous entry
				var (
					deltaTime string
					velocity  float64
				)

				if i > 0 {
					deltaSeconds := entry.Time.Sub(entries[i-1].Time).Seconds()
					deltaTime = fmt.Sprintf("%.0f", deltaSeconds)

					// Calculate temperature velocity in °F/hour
					if deltaSeconds > 0 {
						deltaTemp := float64(entry.Probe - entries[i-1].Probe)
						velocity = deltaTemp / deltaSeconds * 3600 // Convert to °F/hour
					}
				} else {
					deltaTime = "0"
					velocity = 0
				}

				// Update exponential predictor
				if entry.ProbeSet > 0 {
					ep.Update(float64(entry.Probe), entry.Time,
						float64(entry.ProbeSet), float64(entry.Grill), float64(entry.GrillSet))
				}

				if ep.IsInitialized() { //nolint:nestif
					// Get exponential predictor state
					filteredTemp, _ := ep.GetCurrentState()
					uncertainty := ep.GetUncertainty()

					// Calculate ETA from exponential model
					exponentialETA := ep.EstimateTimeToTarget(float64(entry.ProbeSet))

					// Calculate accuracy and actual remaining if we have actual finish time
					var accuracy, actualRemainingStr string

					if actualFinish != nil {
						actualRemaining := actualFinish.Sub(entry.Time)
						if actualRemaining > 0 {
							actualRemainingStr = formatDuration(actualRemaining)
							if exponentialETA > 0 {
								errorPercent := (exponentialETA.Seconds() - actualRemaining.Seconds()) / actualRemaining.Seconds() * 100
								accuracy = fmt.Sprintf("%+.1f%%", errorPercent)
							}
						} else {
							actualRemainingStr = "DONE"
							accuracy = "DONE"
						}
					}

					output.Write(row{
						Time:     entry.Time.Format(time.TimeOnly),
						Delta:    deltaTime,
						Grill:    entry.Grill,
						Probe:    entry.Probe,
						Target:   entry.ProbeSet,
						Velocity: fmt.Sprintf("%.1f", velocity),
						Filtered: fmt.Sprintf("%.2f", filteredTemp),
						ExpETA:   formatDuration(exponentialETA),
						Actual:   actualRemainingStr,
						Accuracy: accuracy,
					})

					// Show prediction details every 10 entries or when accuracy changes significantly
					if i > 0 && (i%10 == 0 || i == len(entries)-1) {
						if actualFinish != nil && exponentialETA > 0 {
							actualRemaining := actualFinish.Sub(entry.Time)
							predictedFinish := entry.Time.Add(exponentialETA)
							expTau := ep.GetTimeConstant()
							output.Annotate(fmt.Sprintf(
								"    -> Predicted finish: %s, Actual remaining: %s, Uncertainty: ±%.1f°F, Tau: %.0fs\n",
								predictedFinish.Format(time.TimeOnly),
								formatDuration(actualRemaining),
								uncertainty,
								expTau))
						}
					}
				} else {
					// First entry - just show initialization
					output.Write(row{
						Time:   entry.Time.Format(time.TimeOnly),
						Delta:  deltaTime,
						Grill:  entry.Grill,
						Probe:  entry.Probe,
						Target: entry.ProbeSet,
					})
				}
			}

			_ = output.Flush()

			// Summary
			fmt.Println()

			if actualFinish != nil {
				firstEntry := entries[0]
				lastEntry := entries[len(entries)-1]
				totalTime := actualFinish.Sub(firstEntry.Time)
				monitoredTime := lastEntry.Time.Sub(firstEntry.Time)

				fmt.Printf("Summary:\n")
				fmt.Printf("  Total cook time: %s\n", formatDuration(totalTime))
				fmt.Printf("  Monitored time: %s\n", formatDuration(monitoredTime))
				fmt.Printf("  Temperature range: %d°F → %d°F (target: %d°F)\n",
					firstEntry.Probe, lastEntry.Probe, lastEntry.ProbeSet)

				// Final prediction accuracy
				if ep.IsInitialized() {
					finalETA := ep.EstimateTimeToTarget(float64(lastEntry.ProbeSet))
					actualRemaining := actualFinish.Sub(lastEntry.Time)

					if actualRemaining > 0 && finalETA > 0 {
						errorPercent := (finalETA.Seconds() - actualRemaining.Seconds()) / actualRemaining.Seconds() * 100
						fmt.Printf("  Final prediction accuracy: %+.1f%% error\n", errorPercent)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&input, "input", "i", "", "input JSON log file")
	cmd.Flags().StringVar(&actualTime, "actual", "",
		"actual finish time (RFC3339 format, e.g., 2025-07-05T20:49:45-04:00)")

	if err := cmd.MarkFlagRequired("input"); err != nil {
		panic(err)
	}

	return &cmd
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}

	return fmt.Sprintf("%dm", minutes)
}
