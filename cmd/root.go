package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"endobit.io/clog"
	"endobit.io/wifire"
)

type Config struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

func newRootCmd() *cobra.Command { //nolint:gocognit
	var (
		output   string
		logLevel string
		debug    bool
		v        *viper.Viper //nolint:varnamelen
	)

	cmd := cobra.Command{
		Use:     "wifire",
		Short:   "Traeger WiFire Grill Util",
		Version: version,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			var level slog.Level

			if err := level.UnmarshalText([]byte(logLevel)); err != nil {
				return fmt.Errorf("invalid log level %q", logLevel)
			}

			opts := clog.HandlerOptions{Level: level}
			slog.SetDefault(slog.New(opts.NewHandler(os.Stderr)))

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			var cfg Config

			if err := v.ReadInConfig(); err != nil {
				slog.Warn("failed to read config file", "error", err)
			}

			if err := v.Unmarshal(&cfg); err != nil {
				return err
			}

			if cfg.Username == "" || cfg.Password == "" {
				return errors.New("username and password must be set, either via flags or config file")
			}

			if debug {
				wifire.Logger = logger
			}

			w, err := wifire.New(wifire.Credentials(cfg.Username, cfg.Password))
			if err != nil {
				return err
			}

			data, err := w.UserData()
			if err != nil {
				return err
			}

			grill := w.NewGrill(data.Things[0].Name)
			if err := grill.Connect(); err != nil {
				return err
			}

			defer grill.Disconnect()

			// Load historical data from file on startup for better ETA stability
			history := []wifire.Status{}
			if output != "" {
				loadedHistory, err := loadHistoricalData(output, 20)
				if err != nil {
					slog.Warn("failed to load historical data", "error", err)
				} else if loadedHistory != nil {
					history = loadedHistory
				}
			}

			// Log startup information if we have historical data
			if len(history) > 0 {
				lastStatus := history[len(history)-1]
				slog.Info("loaded historical data for startup ETA stability",
					"entries", len(history),
					"last_ambient", lastStatus.Ambient,
					"last_grill", lastStatus.Grill,
					"last_probe", lastStatus.Probe)
			}

			if output != "" {
				fout, err := os.OpenFile(output, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
				if err != nil {
					return err
				}

				defer fout.Close()

				go status(grill, fout, history)
			} else {
				go status(grill, nil, history)
			}

			catch := make(chan os.Signal, 1)
			signal.Notify(catch, syscall.SIGINT, syscall.SIGTERM)
			<-catch

			return nil
		},
	}

	v = viper.NewWithOptions(viper.WithLogger(slog.Default()))
	v.AddConfigPath(configFilePath(cmd.Use))
	v.SetConfigName("config")

	info := strings.ToLower(slog.LevelInfo.String())
	cmd.PersistentFlags().StringVar(&logLevel, "log", info, "log level")
	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "debug wifire API")
	cmd.Flags().StringVar(&output, "output", "", "log to file")

	cmd.Flags().String("username", "", "account username")
	cmd.Flags().String("password", "", "account password")

	if err := v.BindPFlag("username", cmd.Flags().Lookup("username")); err != nil {
		panic(err)
	}

	if err := v.BindPFlag("password", cmd.Flags().Lookup("password")); err != nil {
		panic(err)
	}

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newPlotCmd())
	cmd.AddCommand(newForecastCmd())

	return &cmd
}

func configFilePath(appName string) string {
	var baseDir string

	switch runtime.GOOS {
	case "windows":
		baseDir = os.Getenv("AppData")
	default:
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg != "" {
			baseDir = xdg
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return ""
			}

			baseDir = filepath.Join(home, ".config")
		}
	}

	return filepath.Join(baseDir, appName)
}

func status(grill *wifire.Grill, out io.Writer, history []wifire.Status) { //nolint:gocognit
	// Initialize exponential predictor for probe temperature prediction
	exponentialPredictor := wifire.NewExponentialPredictor()

	// Initialize predictor with historical data if available
	if len(history) > 0 {
		slog.Info("initializing predictor with historical data", "entries", len(history))

		for i := range history {
			status := &history[i]

			if status.Probe > 0 && status.ProbeSet > 0 { // Only use valid probe readings
				exponentialPredictor.Update(float64(status.Probe), status.Time,
					float64(status.ProbeSet), float64(status.Grill), float64(status.GrillSet))
			}
		}

		// Log initial predictor state
		temp, velocity := exponentialPredictor.GetCurrentState()
		uncertainty := exponentialPredictor.GetUncertainty()
		slog.Info("exponential predictor initialized",
			"temperature", temp,
			"velocity_deg_per_hour", velocity*3600,
			"uncertainty", uncertainty)
	}

	ch := make(chan wifire.Status, 1)

	if err := grill.SubscribeStatus(ch); err != nil {
		slog.Error("cannot subscribe to status", "error", err)

		return
	}

	for msg := range ch {
		if !msg.Connected {
			slog.Warn("grill disconnected")

			continue
		}

		if msg.Error != nil {
			slog.Error("invalid status", "error", msg.Error)
		}

		// Update predictor with new probe measurement
		if msg.Probe > 0 && msg.ProbeSet > 0 {
			exponentialPredictor.Update(float64(msg.Probe), msg.Time,
				float64(msg.ProbeSet), float64(msg.Grill), float64(msg.GrillSet))
		}

		history = append(history, msg)
		if len(history) > 5 {
			history = history[1:]
		}

		attrs := []slog.Attr{
			slog.String("status", msg.SystemStatus.String()),
			slog.String("units", msg.Units.String()),
			slog.Int("ambient", msg.Ambient),
			slog.Int("grill", msg.Grill),
			slog.Int("grill_set", msg.GrillSet),
		}

		if msg.ProbeConnected {
			attrs = append(attrs,
				slog.Int("probe", msg.Probe),
				slog.Int("probe_set", msg.ProbeSet),
				slog.Bool("probe_alarm", msg.ProbeAlarmFired))
		}

		// Calculate ETA using exponential prediction model
		if msg.ProbeSet > 0 && msg.Probe < msg.ProbeSet { //nolint:nestif
			var (
				bestETA, exponentialETA time.Duration
				etaSource               string
			)

			// Get exponential predictor prediction
			if exponentialPredictor.IsInitialized() {
				exponentialETA = exponentialPredictor.EstimateTimeToTarget(float64(msg.ProbeSet))
			}

			// Use exponential predictor as primary, fallback to legacy calculation
			if exponentialETA > 0 && exponentialETA < 24*time.Hour {
				bestETA = exponentialETA
				etaSource = "exponential"
			} else {
				// Fallback to original calculation method
				bestETA = calculateProbeETA(history, &msg)
				etaSource = "legacy"
			}

			if bestETA > 0 {
				msg.ProbeETA = wifire.JSONDuration(bestETA)
				attrs = append(attrs,
					slog.Duration("probe_eta", bestETA.Round(time.Minute)),
					slog.String("eta_source", etaSource))

				// Add detailed predictor state to debug logging
				if wifire.Logger != nil {
					// Exponential predictor state
					eTemp, eVelocity := exponentialPredictor.GetCurrentState()
					eUncertainty := exponentialPredictor.GetUncertainty()
					eTau := exponentialPredictor.GetTimeConstant()

					wifire.Logger(wifire.LogDebug, "eta_models", fmt.Sprintf(
						"source=%s, exp_eta=%.1fm (temp=%.2f, vel=%.2f°/hr, unc=%.2f, tau=%.0fs), final_eta=%.1fm",
						etaSource, exponentialETA.Minutes(), eTemp, eVelocity*3600, eUncertainty, eTau,
						bestETA.Minutes()))
				}
			}
		}

		slog.LogAttrs(context.TODO(), slog.LevelInfo, "", attrs...)

		if out != nil {
			b, err := json.Marshal(msg)
			if err != nil {
				slog.Error("cannot marshal", "error", err)
			}

			_, _ = out.Write(b)
			_, _ = out.Write([]byte("\n"))
		}
	}
}

// loadHistoricalData reads existing JSON data from the output file to initialize history
func loadHistoricalData(filename string, maxEntries int) ([]wifire.Status, error) {
	if filename == "" {
		return nil, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		// File doesn't exist yet - not an error for new files
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}
	defer file.Close()

	var history []wifire.Status

	scanner := bufio.NewScanner(file)

	// Read all lines first to get the most recent entries
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Parse the most recent entries (up to maxEntries)
	startIdx := 0
	if len(lines) > maxEntries {
		startIdx = len(lines) - maxEntries
	}

	for i := startIdx; i < len(lines); i++ {
		var status wifire.Status
		if err := json.Unmarshal([]byte(lines[i]), &status); err != nil {
			// Skip invalid lines but continue processing
			slog.Warn("skipping invalid JSON line in history file", "line", i+1, "error", err)

			continue
		}

		// Only include entries with valid probe data and recent timestamps (last 2 hours)
		if status.Time.After(time.Now().Add(-2*time.Hour)) && status.Probe > 0 {
			history = append(history, status)
		}
	}

	if len(history) > 0 {
		slog.Info("loaded historical data for startup", "entries", len(history),
			"oldest", history[0].Time.Format("15:04:05"),
			"newest", history[len(history)-1].Time.Format("15:04:05"))
	}

	return history, nil
}

// calculateProbeETA estimates time to reach target probe temperature using multiple factors
func calculateProbeETA(history []wifire.Status, current *wifire.Status) time.Duration {
	if len(history) < 1 {
		return 0
	}

	// Use the oldest entry in history and current message for rate calculation
	first := history[0]
	last := current // Use current message as the most recent data point

	// Ensure we have valid time progression
	if !last.Time.After(first.Time) {
		return 0
	}

	// Calculate basic rate from first historical entry to current
	tempChange := float64(last.Probe - first.Probe)
	timeChange := last.Time.Sub(first.Time).Seconds()

	var (
		baseRate  float64
		tempTrend string
	)

	switch {
	case tempChange > 0:
		// Temperature is rising - normal calculation
		baseRate = tempChange / timeChange
		tempTrend = "rising"
	case tempChange < 0:
		// Temperature is falling - return a very long ETA to indicate issue
		tempTrend = "falling"
		// Use a very slow hypothetical rate (1 degree per hour) for falling temp
		baseRate = 1.0 / 3600.0 // 1°F per hour in degrees per second
	default:
		// Temperature is stable/plateau - use a very slow rate
		tempTrend = "stable"
		// Use a minimal rate to indicate very long ETA for stable temps
		baseRate = 0.5 / 3600.0 // 0.5°F per hour in degrees per second
	}

	// Calculate recent rate using last few history entries plus current
	var recentRate float64

	if len(history) >= 2 {
		// Use last 2 history entries plus current for recent trend
		recentStart := history[len(history)-2]
		recentTempChange := float64(current.Probe - recentStart.Probe)
		recentTimeChange := current.Time.Sub(recentStart.Time).Seconds()

		if recentTimeChange > 0 {
			switch {
			case recentTempChange > 0:
				recentRate = recentTempChange / recentTimeChange
			case recentTempChange < 0:
				// Recent falling trend - use slow rate
				recentRate = 1.0 / 3600.0
			default:
				// Recent stable trend - use minimal rate
				recentRate = 0.5 / 3600.0
			}
		}
	}

	// Debug logging for temperature trends
	if wifire.Logger != nil {
		wifire.Logger(wifire.LogDebug, "eta", fmt.Sprintf(
			"Temp trend: %s, temp_change=%.2f°F over %.0fs, base_rate=%.6f°F/s, current_time=%s",
			tempTrend, tempChange, timeChange, baseRate, current.Time.Format("15:04:05")))
	}

	// Weight recent vs historical rates (recent conditions are more relevant)
	rate := baseRate
	if recentRate > 0 {
		rate = (recentRate * 0.7) + (baseRate * 0.3)
	}

	// Factor 1: Grill temperature state adjustment
	grillTempAdjustment := 1.0
	grillHeatingUp := current.Grill < current.GrillSet
	grillOvershoot := current.Grill > current.GrillSet

	if grillHeatingUp {
		// If grill is still heating up, probe will heat faster initially
		grillTempAdjustment = 1.15
	} else if grillOvershoot {
		// If grill is overshooting, heating may be less predictable
		grillTempAdjustment = 0.95
	}

	// Factor 2: Temperature differential between grill and probe
	tempDifferential := float64(current.Grill - current.Probe)
	differentialAdjustment := 1.0

	switch {
	case tempDifferential > 50:
		// Large differential = faster heating
		differentialAdjustment = 1.1
	case tempDifferential < 20:
		// Small differential = slower heating (approaching equilibrium)
		differentialAdjustment = 0.8
	case tempDifferential < 10:
		// Very small differential = much slower heating
		differentialAdjustment = 0.6
	}

	// Factor 3: Cooking stage adjustment (early/middle/late)
	cookingProgress := float64(current.Probe) / float64(current.ProbeSet)
	stageAdjustment := 1.0

	if cookingProgress < 0.3 {
		// Early stage - typically faster heating
		stageAdjustment = 1.05
	} else if cookingProgress > 0.8 {
		// Late stage - typically slower as approaching target
		stageAdjustment = 0.85
	}
	// Middle stage uses base rate (adjustment = 1.0)

	// Factor 4: Rate stability check
	// If recent rate is much different from historical, be more conservative
	stabilityAdjustment := 1.0

	if recentRate > 0 && baseRate > 0 {
		rateRatio := recentRate / baseRate
		if rateRatio > 1.5 || rateRatio < 0.5 {
			// Unstable rate - be more conservative
			stabilityAdjustment = 0.9
		}
	}

	// Apply all adjustments
	adjustedRate := rate * grillTempAdjustment * differentialAdjustment * stageAdjustment * stabilityAdjustment

	// Debug logging for adjustment factors (only if wifire debug logging is enabled)
	if wifire.Logger != nil {
		wifire.Logger(wifire.LogDebug, "eta",
			fmt.Sprintf("ETA calc: base_rate=%.4f, recent_rate=%.4f, grill_adj=%.2f, diff_adj=%.2f, "+
				"stage_adj=%.2f, stability_adj=%.2f, final_rate=%.4f",
				baseRate, recentRate, grillTempAdjustment, differentialAdjustment,
				stageAdjustment, stabilityAdjustment, adjustedRate))
	}

	// Calculate ETA
	tempRemaining := float64(current.ProbeSet - current.Probe)
	etaSeconds := tempRemaining / adjustedRate

	// Enhanced sanity checks and messaging
	if etaSeconds < 0 {
		return 0
	}

	if etaSeconds > 24*3600 { // More than 24 hours
		// For very long ETAs, cap at 24 hours to indicate "a very long time"
		// This happens with stable/falling temps or very slow heating
		if wifire.Logger != nil {
			wifire.Logger(wifire.LogDebug, "eta", fmt.Sprintf(
				"ETA >24h (%.1fh calculated) - temp trend: %s, capping at 24h",
				etaSeconds/3600, tempTrend))
		}

		return 24 * time.Hour
	}

	// Log successful ETA calculation
	if wifire.Logger != nil {
		wifire.Logger(wifire.LogDebug, "eta", fmt.Sprintf(
			"ETA calculated: %.1f minutes (temp trend: %s)",
			etaSeconds/60, tempTrend))
	}

	return time.Duration(etaSeconds) * time.Second
}
