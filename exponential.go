package wifire

import (
	"math"
	"time"
)

// ExponentialPredictor implements an exponential approach model for cooking temperature prediction
// Based on the physics that food temperature approaches an equilibrium temperature exponentially
// Considers grill temperature, grill set point, and heat transfer dynamics
type ExponentialPredictor struct {
	// Model parameters
	targetTemp   float64
	startTemp    float64
	timeConstant float64 // τ (tau) - how quickly we approach equilibrium
	startTime    time.Time

	// Environmental factors
	grillTemp    float64 // Current grill temperature
	grillSetTemp float64 // Target grill temperature

	// Data points for parameter estimation
	temperatures  []float64
	grillTemps    []float64
	grillSetTemps []float64
	timestamps    []time.Time

	// State
	initialized   bool
	minDataPoints int
}

// NewExponentialPredictor creates a new exponential approach predictor
func NewExponentialPredictor() *ExponentialPredictor {
	return &ExponentialPredictor{
		minDataPoints: 3, // Need at least 3 points to estimate tau
		initialized:   false,
	}
}

// Update processes a new temperature measurement and updates the model
// Now considers grill temperature and grill set point for more accurate prediction
func (ep *ExponentialPredictor) Update(probeTemp float64, timestamp time.Time, probeTargetTemp, grillTemp, grillSetTemp float64) { //nolint:lll
	if !ep.initialized {
		// First measurement - initialize
		ep.startTemp = probeTemp
		ep.targetTemp = probeTargetTemp
		ep.grillTemp = grillTemp
		ep.grillSetTemp = grillSetTemp
		ep.startTime = timestamp
		ep.temperatures = []float64{probeTemp}
		ep.grillTemps = []float64{grillTemp}
		ep.grillSetTemps = []float64{grillSetTemp}
		ep.timestamps = []time.Time{timestamp}
		ep.timeConstant = 3600.0 // Default: 1 hour time constant
		ep.initialized = true

		return
	}

	// Update current values
	ep.targetTemp = probeTargetTemp
	ep.grillTemp = grillTemp
	ep.grillSetTemp = grillSetTemp

	// Add new data points
	ep.temperatures = append(ep.temperatures, probeTemp)
	ep.grillTemps = append(ep.grillTemps, grillTemp)
	ep.grillSetTemps = append(ep.grillSetTemps, grillSetTemp)
	ep.timestamps = append(ep.timestamps, timestamp)

	// Keep only recent data (last 20 points to avoid memory bloat)
	if len(ep.temperatures) > 20 {
		ep.temperatures = ep.temperatures[1:]
		ep.grillTemps = ep.grillTemps[1:]
		ep.grillSetTemps = ep.grillSetTemps[1:]
		ep.timestamps = ep.timestamps[1:]
	}

	// Re-estimate time constant if we have enough data
	if len(ep.temperatures) >= ep.minDataPoints {
		ep.estimateTimeConstant()
	}
}

// PredictTemperature predicts the temperature at a future time using grill dynamics
func (ep *ExponentialPredictor) PredictTemperature(futureTime time.Time) float64 {
	// Use current data index (most recent)
	currentIdx := len(ep.temperatures) - 1

	return ep.predictAtTimeWithGrillDynamics(futureTime, ep.timeConstant, currentIdx)
}

// EstimateTimeToTarget estimates the time required to reach a target temperature using grill dynamics
func (ep *ExponentialPredictor) EstimateTimeToTarget(targetTemp float64) time.Duration {
	if !ep.initialized {
		return 0
	}

	// If we're already at or past target, return 0
	currentTemp := ep.getCurrentTemperature()
	if currentTemp >= targetTemp {
		return 0
	}

	// Calculate effective equilibrium temperature considering grill dynamics
	effectiveEquilibrium := ep.calculateEffectiveEquilibrium(ep.grillTemp, ep.grillSetTemp, targetTemp)

	// Check if target is achievable with current grill conditions
	if targetTemp > effectiveEquilibrium {
		// Target is higher than what grill conditions can achieve
		// Return time to reach the maximum achievable temperature instead
		if currentTemp >= effectiveEquilibrium {
			return 0 // Already at max achievable
		}
		// Use effective equilibrium as target for calculation
		targetTemp = effectiveEquilibrium
	}

	// Solve for t when T(t) = targetTemp using grill-aware model
	// targetTemp = effectiveEquilibrium - (effectiveEquilibrium - startTemp) * exp(-t/τ)
	// Rearranging: exp(-t/τ) = (targetTemp - effectiveEquilibrium) / (startTemp - effectiveEquilibrium)
	// Therefore: t = -τ * ln((targetTemp - effectiveEquilibrium) / (startTemp - effectiveEquilibrium))

	numerator := targetTemp - effectiveEquilibrium
	denominator := ep.startTemp - effectiveEquilibrium

	// For a valid solution, we need:
	// 1. denominator != 0 (start temp != equilibrium)
	// 2. The ratio must be positive (for valid logarithm)
	// 3. The ratio must be < 1 (target closer to equilibrium than start)

	if math.Abs(denominator) < 0.1 {
		// Start temperature too close to equilibrium, use linear fallback
		return ep.linearTimeToTarget(targetTemp)
	}

	ratio := numerator / denominator

	if ratio <= 0 || ratio >= 1 {
		// Invalid ratio for exponential model, use linear fallback
		return ep.linearTimeToTarget(targetTemp)
	}

	timeToTarget := -ep.timeConstant * math.Log(ratio)

	// Subtract elapsed time to get remaining time
	// Use the most recent timestamp from data, not real current time (for forecast compatibility)
	currentTime := ep.timestamps[len(ep.timestamps)-1]
	elapsed := currentTime.Sub(ep.startTime).Seconds()
	remaining := timeToTarget - elapsed

	if remaining < 0 {
		remaining = 0
	}

	// Cap at reasonable maximum (8 hours)
	maxSeconds := 8 * 3600.0
	if remaining > maxSeconds {
		remaining = maxSeconds
	}

	return time.Duration(remaining * float64(time.Second))
}

// GetCurrentState returns the current filtered temperature and velocity
func (ep *ExponentialPredictor) GetCurrentState() (temperature, velocity float64) {
	if !ep.initialized {
		return 0, 0
	}

	currentTemp := ep.getCurrentTemperature()

	// Calculate current velocity: dT/dt at current time
	// For exponential model: dT/dt = (T_target - T_start) * (1/τ) * exp(-t/τ)
	elapsed := time.Since(ep.startTime).Seconds()
	velocity = (ep.targetTemp - ep.startTemp) / ep.timeConstant * math.Exp(-elapsed/ep.timeConstant)

	return currentTemp, velocity
}

// GetUncertainty returns an estimate of prediction uncertainty
func (ep *ExponentialPredictor) GetUncertainty() float64 {
	if !ep.initialized || len(ep.temperatures) < 3 {
		return 5.0 // High uncertainty
	}

	// Calculate recent prediction errors
	recentErrors := make([]float64, 0)

	for i := len(ep.temperatures) - 5; i < len(ep.temperatures); i++ {
		if i < 0 {
			continue
		}

		predicted := ep.predictAtTime(ep.timestamps[i], ep.timeConstant)
		actual := ep.temperatures[i]
		recentErrors = append(recentErrors, math.Abs(predicted-actual))
	}

	if len(recentErrors) == 0 {
		return 2.0
	}

	// Return average absolute error as uncertainty
	sum := 0.0
	for _, err := range recentErrors {
		sum += err
	}

	return sum / float64(len(recentErrors))
}

// IsInitialized returns whether the predictor has been initialized
func (ep *ExponentialPredictor) IsInitialized() bool {
	return ep.initialized
}

// GetTimeConstant returns the current time constant (for debugging)
func (ep *ExponentialPredictor) GetTimeConstant() float64 {
	return ep.timeConstant
}

// estimateTimeConstant estimates τ from the available data using least squares
// Now considers grill temperature dynamics for more accurate modeling
func (ep *ExponentialPredictor) estimateTimeConstant() {
	if len(ep.temperatures) < ep.minDataPoints {
		return
	}

	// Use recent data for estimation (last 10 points)
	startIdx := 0
	if len(ep.temperatures) > 10 {
		startIdx = len(ep.temperatures) - 10
	}

	// Try different time constants and find the one with best fit
	bestTau := ep.timeConstant
	bestError := math.Inf(1)

	// Search from 5 minutes to 8 hours
	for tau := 300.0; tau <= 28800.0; tau += 300.0 { // Step by 5 minutes
		error := ep.calculateErrorWithGrillDynamics(tau, startIdx) //nolint:gocritic
		if error < bestError {
			bestError = error
			bestTau = tau
		}
	}

	// Only update if the improvement is significant and tau is reasonable
	if bestError < 0.9*ep.calculateErrorWithGrillDynamics(ep.timeConstant, startIdx) {
		ep.timeConstant = bestTau
	}
}

// calculateError calculates the mean squared error for a given time constant
// func (ep *ExponentialPredictor) calculateError(tau float64, startIdx int) float64 {
// 	sumSquaredError := 0.0
// 	count := 0

// 	for i := startIdx; i < len(ep.temperatures); i++ {
// 		predicted := ep.predictAtTime(ep.timestamps[i], tau)
// 		actual := ep.temperatures[i]
// 		error := predicted - actual
// 		sumSquaredError += error * error
// 		count++
// 	}

// 	if count == 0 {
// 		return math.Inf(1)
// 	}

// 	return sumSquaredError / float64(count)
// }

// calculateErrorWithGrillDynamics calculates the mean squared error considering grill temperature effects
func (ep *ExponentialPredictor) calculateErrorWithGrillDynamics(tau float64, startIdx int) float64 {
	sumSquaredError := 0.0
	count := 0

	for i := startIdx; i < len(ep.temperatures); i++ {
		predicted := ep.predictAtTimeWithGrillDynamics(ep.timestamps[i], tau, i)
		actual := ep.temperatures[i]
		error := predicted - actual //nolint:gocritic
		sumSquaredError += error * error
		count++
	}

	if count == 0 {
		return math.Inf(1)
	}

	return sumSquaredError / float64(count)
}

// predictAtTime predicts temperature at a specific time using given tau
func (ep *ExponentialPredictor) predictAtTime(t time.Time, tau float64) float64 {
	if !ep.initialized {
		return 0
	}

	elapsed := t.Sub(ep.startTime).Seconds()
	if elapsed < 0 {
		return ep.startTemp
	}

	// T(t) = T_target - (T_target - T_start) * exp(-t/τ)
	return ep.targetTemp - (ep.targetTemp-ep.startTemp)*math.Exp(-elapsed/tau)
}

// predictAtTimeWithGrillDynamics predicts temperature considering grill temperature effects
func (ep *ExponentialPredictor) predictAtTimeWithGrillDynamics(t time.Time, tau float64, dataIdx int) float64 {
	if !ep.initialized {
		return 0
	}

	elapsed := t.Sub(ep.startTime).Seconds()
	if elapsed < 0 {
		return ep.startTemp
	}

	// Calculate effective equilibrium temperature based on grill dynamics
	// The probe doesn't just approach the probe target, but is influenced by grill temperature
	var grillTemp, grillSetTemp float64
	if dataIdx < len(ep.grillTemps) {
		grillTemp = ep.grillTemps[dataIdx]
		grillSetTemp = ep.grillSetTemps[dataIdx]
	} else {
		grillTemp = ep.grillTemp
		grillSetTemp = ep.grillSetTemp
	}

	// Physics-based effective equilibrium calculation
	// The probe temperature is influenced by both the grill chamber temperature and the target
	// Factor in heat transfer efficiency and grill stability
	effectiveEquilibrium := ep.calculateEffectiveEquilibrium(grillTemp, grillSetTemp, ep.targetTemp)

	// T(t) = T_equilibrium - (T_equilibrium - T_start) * exp(-t/τ)
	return effectiveEquilibrium - (effectiveEquilibrium-ep.startTemp)*math.Exp(-elapsed/tau)
}

// calculateEffectiveEquilibrium determines the equilibrium temperature considering grill dynamics
func (ep *ExponentialPredictor) calculateEffectiveEquilibrium(grillTemp, grillSetTemp, probeTarget float64) float64 {
	// Physics: The probe equilibrium is influenced by:
	// 1. The grill chamber temperature (primary heat source)
	// 2. The probe target (what we want to achieve)
	// 3. Heat transfer efficiency and thermal mass

	// Temperature differential between grill and probe target
	grillDelta := grillTemp - probeTarget

	// If grill is much hotter than target, use grill temp but moderated
	// If grill is close to target, the probe can reach close to target
	// If grill is cooler than target, the probe won't reach target

	var effectiveEquilibrium float64

	switch {
	case grillDelta > 50:
		// Grill much hotter than target - probe can reach target but limited by heat transfer
		// Use target but allow some overshoot if grill is very hot
		effectiveEquilibrium = probeTarget + math.Min(grillDelta*0.1, 10)
	case grillDelta > 20:
		// Grill moderately hotter - good conditions for reaching target
		effectiveEquilibrium = probeTarget + grillDelta*0.2
	case grillDelta > 0:
		// Grill slightly hotter - can reach target
		effectiveEquilibrium = probeTarget + grillDelta*0.5
	default:
		// Grill at or below target - probe won't reach full target
		// This models heat loss and thermal inefficiency
		effectiveEquilibrium = grillTemp + math.Max(grillDelta*0.3, -20)
	}

	// Factor in grill stability (how close grill is to its set point)
	grillStability := 1.0 - math.Abs(grillTemp-grillSetTemp)/50.0
	if grillStability < 0.5 {
		grillStability = 0.5 // Minimum stability factor
	}

	// Adjust equilibrium based on grill stability
	effectiveEquilibrium = probeTarget + (effectiveEquilibrium-probeTarget)*grillStability

	return effectiveEquilibrium
}

// linearTimeToTarget provides a fallback linear estimation
func (ep *ExponentialPredictor) linearTimeToTarget(targetTemp float64) time.Duration {
	if len(ep.temperatures) < 2 {
		return 0
	}

	// Calculate rate from last few points
	recentPoints := 5
	if len(ep.temperatures) < recentPoints {
		recentPoints = len(ep.temperatures)
	}

	startIdx := len(ep.temperatures) - recentPoints
	timeDiff := ep.timestamps[len(ep.timestamps)-1].Sub(ep.timestamps[startIdx]).Seconds()
	tempDiff := ep.temperatures[len(ep.temperatures)-1] - ep.temperatures[startIdx]

	if timeDiff <= 0 || tempDiff <= 0 {
		return 0
	}

	rate := tempDiff / timeDiff // degrees per second
	currentTemp := ep.temperatures[len(ep.temperatures)-1]
	remaining := targetTemp - currentTemp

	if remaining <= 0 {
		return 0
	}

	return time.Duration(remaining/rate) * time.Second
}

// getCurrentTemperature returns the most recent temperature
func (ep *ExponentialPredictor) getCurrentTemperature() float64 {
	if !ep.initialized || len(ep.temperatures) == 0 {
		return 0
	}

	return ep.temperatures[len(ep.temperatures)-1]
}
