package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"time"

	"endobit.io/app/log"
	"endobit.io/wifire"
)

type monitor struct {
	Logger  *slog.Logger
	Grill   *wifire.Client
	Output  io.Writer
	History []wifire.Status
}

func (m *monitor) Run(ctx context.Context, grillName string) error { //nolint:gocognit
	// Initialize exponential predictor for probe temperature prediction
	exponentialPredictor := wifire.NewExponentialPredictor()

	// Initialize predictor with historical data if available
	if len(m.History) > 0 {
		m.Logger.Info("initializing predictor with historical data", "entries", len(m.History))

		for i := range m.History {
			status := &m.History[i]

			if status.Probe > 0 && status.ProbeSet > 0 { // Only use valid probe readings
				exponentialPredictor.Update(float64(status.Probe), status.Time,
					float64(status.ProbeSet), float64(status.Grill), float64(status.GrillSet))
			}
		}

		// Log initial predictor state
		temp, velocity := exponentialPredictor.GetCurrentState()
		uncertainty := exponentialPredictor.GetUncertainty()
		m.Logger.Info("exponential predictor initialized",
			"temperature", temp,
			"velocity_deg_per_hour", velocity*3600,
			"uncertainty", uncertainty)
	}

	subscription := make(chan wifire.Status, 1)

	var ticker *time.Ticker

	defer func() {
		if ticker != nil {
			ticker.Stop()
		}

		if m.Grill != nil {
			m.Grill.MQTTDisconnect()
		}
	}()

	for {
		ticker = time.NewTicker(1 * time.Minute)

		if !m.Grill.MQTTIsConnected() {
			if err := m.Grill.MQTTConnect(); err != nil {
				m.Logger.Error("cannot connect to MQTT", "error", err)
			} else {
				if err := m.Grill.MQTTSubscribeStatus(grillName, subscription); err != nil {
					m.Logger.Error("cannot subscribe to status", "error", err)
					m.Grill.MQTTDisconnect()
				}
			}
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			m.Logger.Info("interrupted, aborting")

			return ctx.Err()

		case msg := <-subscription:
			if !msg.Connected {
				m.Logger.Warn("grill disconnected")

				continue
			}

			if msg.Error != nil {
				m.Logger.Error("invalid status", "error", msg.Error)
			}

			// Update predictor with new probe measurement
			if msg.Probe > 0 && msg.ProbeSet > 0 {
				exponentialPredictor.Update(float64(msg.Probe), msg.Time,
					float64(msg.ProbeSet), float64(msg.Grill), float64(msg.GrillSet))
			}

			m.History = append(m.History, msg)
			if len(m.History) > 5 {
				m.History = m.History[1:]
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
			if msg.ProbeSet > 0 && msg.Probe < msg.ProbeSet {
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
					bestETA = m.calculateProbeETA(&msg)
					etaSource = "legacy"
				}

				if bestETA > 0 {
					msg.ProbeETA = wifire.JSONDuration(bestETA)
					attrs = append(attrs,
						slog.Duration("probe_eta", bestETA.Round(time.Minute)),
						slog.String("eta_source", etaSource))

					// Add detailed predictor state to debug logging
					// Exponential predictor state
					eTemp, eVelocity := exponentialPredictor.GetCurrentState()
					eUncertainty := exponentialPredictor.GetUncertainty()
					eTau := exponentialPredictor.GetTimeConstant()

					m.Logger.Debug("eta_models",
						slog.String("source", etaSource),
						log.Format("%.1f", "exp_eta", exponentialETA.Minutes()),
						log.Format("%.2f", "temp", eTemp),
						log.Format("%.2f°/hr", "vel", eVelocity*3600),
						log.Format("%.2f", "unc", eUncertainty),
						log.Format("%.0fs", "tau", eTau),
						log.Format("%.1fm", "final_eta", bestETA.Minutes()))
				}
			}

			m.Logger.LogAttrs(context.TODO(), slog.LevelInfo, "", attrs...)

			if m.Output != nil {
				b, err := json.Marshal(msg)
				if err != nil {
					m.Logger.Error("cannot marshal", "error", err)
				}

				_, _ = m.Output.Write(b)
				_, _ = m.Output.Write([]byte("\n"))
			}
		}
	}
}

// calculateProbeETA estimates time to reach target probe temperature using multiple factors
func (m *monitor) calculateProbeETA(current *wifire.Status) time.Duration {
	if len(m.History) < 1 {
		return 0
	}

	// Use the oldest entry in history and current message for rate calculation
	first := m.History[0]
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

	if len(m.History) >= 2 {
		// Use last 2 history entries plus current for recent trend
		recentStart := m.History[len(m.History)-2]
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
	m.Logger.Debug("eta",
		slog.String("temp_trend", tempTrend),
		log.Format("%.2f°", "temp_change", tempChange),
		log.Format("%.0fs", "over", timeChange),
		log.Format("%.6f°/s", "base_rate", baseRate),
		slog.String("current_time", current.Time.Format(time.TimeOnly)))

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
	m.Logger.Debug("eta",
		log.Format("%.4f", "base_rate", baseRate),
		log.Format("%.4f", "recent_rate", recentRate),
		log.Format("%.2f", "grill_adj", grillTempAdjustment),
		log.Format("%.2f", "diff_adj", differentialAdjustment),
		log.Format("%.2f", "stage_adj", stageAdjustment),
		log.Format("%.2f", "stability_adj", stabilityAdjustment),
		log.Format("%.4f°/s", "final_rate", adjustedRate))

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
		m.Logger.Debug("eta > 24h",
			log.Format("%.1fh", "calculated", etaSeconds/3600),
			slog.String("temp_trend", tempTrend),
			slog.String("capping", "24h"))

		return 24 * time.Hour
	}

	// Log successful ETA calculation
	m.Logger.Debug("eta",
		log.Format("%.1fm", "calculated", etaSeconds/60),
		slog.String("temp_trend", tempTrend))

	return time.Duration(etaSeconds) * time.Second
}
