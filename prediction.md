# Exponential Temperature Prediction System

## Overview

This system predicts cooking times using an **exponential approach model** based on the physics of heat transfer during cooking. 

https://xkcd.com/612/
https://www.youtube.com/watch?v=9gTLDuxmQek


## Physics Background

### Heat Transfer in Cooking

When you cook meat, the temperature doesn't rise linearly. Instead, it follows an **exponential approach** toward an equilibrium temperature. This happens because:

1. **Newton's Law of Cooling**: The rate of temperature change is proportional to the temperature difference between the meat and its environment
2. **Thermal Mass**: Larger pieces of meat heat up more slowly
3. **Heat Capacity**: Different materials require different amounts of energy to change temperature
4. **Surface Area**: More surface area = faster heat transfer

### The Exponential Model

The temperature follows this equation:
```
T(t) = T_equilibrium - (T_equilibrium - T_start) × e^(-t/τ)
```

Where:
- `T(t)` = temperature at time t
- `T_equilibrium` = final equilibrium temperature the meat approaches
- `T_start` = starting temperature
- `τ` (tau) = time constant (how fast the meat heats up)
- `t` = elapsed time

### What is Tau (τ)?

**Tau is the time it takes to reach 63.2% of the way to equilibrium.** Think of it as the "cooking speed":

- **Small tau** (fast cooking): Thin chicken breast, high grill temp
- **Large tau** (slow cooking): Thick brisket, low-and-slow temperatures

**Examples:**
- Tau = 30 minutes: After 30 min → 63% done, after 60 min → 86% done, after 90 min → 95% done
- Tau = 2 hours: After 2 hours → 63% done, after 4 hours → 86% done, after 6 hours → 95% done

## System Architecture

### ExponentialPredictor Class

The predictor maintains:
- **Historical data**: Recent temperature, grill temp, and time measurements
- **Model parameters**: Current tau estimate, start/target temperatures
- **Environmental factors**: Grill temperature and setpoint for physics-based corrections

### Key Methods

1. **Update()**: Processes new temperature readings and re-estimates tau
2. **EstimateTimeToTarget()**: Predicts time to reach target temperature
3. **estimateTimeConstant()**: Finds the best tau by testing against historical data
4. **calculateEffectiveEquilibrium()**: Adjusts target based on grill conditions

## Physics-Based Grill Dynamics

The system goes beyond simple exponential curves by considering **real grill physics**:

### Effective Equilibrium Temperature

The meat doesn't just approach the probe target—it's influenced by grill temperature:

```
Effective Equilibrium = f(grill_temp, grill_setpoint, probe_target)
```

**Examples:**
- Grill at 250°F, target 204°F → meat can reach ~204°F
- Grill at 180°F, target 204°F → meat might only reach ~185°F (insufficient heat)
- Grill at 350°F, target 204°F → meat might overshoot to ~210°F (too much heat)

### Grill Stability Factor

Unstable grill temperatures reduce prediction accuracy:
```
stability = 1.0 - |grill_temp - grill_setpoint| / 50.0
```

## Tunable Parameters

### Core Model Parameters

| Parameter | Default | Range | Purpose |
|-----------|---------|-------|---------|
| `minDataPoints` | 3 | 2-10 | Minimum measurements before estimating tau |
| `maxHistory` | 20 | 10-50 | How many recent points to keep |
| `defaultTau` | 3600s (1hr) | 300-28800s | Initial tau estimate |

### Tau Search Parameters

| Parameter | Default | Range | Purpose |
|-----------|---------|-------|---------|
| `tauMin` | 300s (5min) | 60-1800s | Fastest possible cooking |
| `tauMax` | 28800s (8hr) | 3600-86400s | Slowest possible cooking |
| `tauStep` | 300s (5min) | 30-600s | Search granularity |

### Grill Dynamics Parameters

| Parameter | Default | Purpose |
|-----------|---------|---------|
| Very hot grill factor | 0.1 | When grill >50°F above target |
| Moderate hot factor | 0.2 | When grill 20-50°F above target |
| Slightly hot factor | 0.5 | When grill 0-20°F above target |
| Cool grill factor | 0.3 | When grill below target |
| Stability divisor | 50.0 | How much grill instability matters |
| Min stability | 0.5 | Prevents extreme instability penalties |

### Convergence Parameters

| Parameter | Default | Purpose |
|-----------|---------|---------|
| Error improvement | 0.9 | New tau must be 10% better to update |
| Max prediction time | 8 hours | Caps unrealistic predictions |
| Equilibrium threshold | 0.1°F | When start temp too close to target |

## Experimental Tuning

### For Faster Response (More Aggressive)
- Reduce `minDataPoints` to 2
- Reduce `maxHistory` to 10
- Increase `tauStep` to 60s for coarser search
- Reduce error improvement threshold to 0.8

### For More Stability (Conservative)
- Increase `minDataPoints` to 5
- Increase `maxHistory` to 30
- Reduce `tauStep` to 60s for finer search
- Increase error improvement threshold to 0.95

### For Different Cooking Styles

**High-Heat/Fast Cooking:**
- Reduce `tauMin` to 180s (3 minutes)
- Reduce `tauMax` to 7200s (2 hours)
- Increase grill factors (more aggressive heat transfer)

**Low-and-Slow:**
- Increase `tauMin` to 600s (10 minutes)
- Increase `tauMax` to 43200s (12 hours)
- Reduce grill factors (gentler heat transfer)

**Precision Cooking:**
- Reduce `tauStep` to 60s
- Increase `maxHistory` to 50
- Increase `minDataPoints` to 8

## Code Locations

### Primary Implementation
- `exponential.go`: Main predictor logic
- `estimateTimeConstant()`: Tau calculation (lines ~85-110)
- `calculateEffectiveEquilibrium()`: Grill dynamics (lines ~200-240)

### Key Constants to Modify
```go
// In NewExponentialPredictor()
minDataPoints: 3,        // Line 36

// In Update()
ep.timeConstant = 3600.0 // Line 55 (default tau)

// In estimateTimeConstant()
for tau := 300.0; tau <= 28800.0; tau += 300.0 // Line 103

// In calculateEffectiveEquilibrium()
effectiveEquilibrium = probeTarget + grillDelta*0.1 // Line 222
effectiveEquilibrium = probeTarget + grillDelta*0.2 // Line 225
effectiveEquilibrium = probeTarget + grillDelta*0.5 // Line 228
effectiveEquilibrium = grillTemp + math.Max(grillDelta*0.3, -20) // Line 232
```

## Testing Your Changes

### Using the Forecast Command
Test parameter changes against historical data:
```bash
./wifire forecast --input your_cook.json --actual 2025-01-17T20:49:45-05:00
```

### Key Metrics to Watch
1. **Accuracy**: How close predictions are to actual finish time
2. **Stability**: How much predictions jump between measurements
3. **Convergence**: How quickly the model settles on accurate predictions
4. **Tau Values**: Are estimated tau values reasonable for your cooking style?

### Sample Output Analysis
```
Time       Delta    Grill    Probe    Target   Velocity   Filtered   Exp ETA      Actual       Accuracy  
19:31:34   61       254      190      204      59.0       190.00     1h17m        1h18m        -0.6%     
```

- **Good accuracy**: ±5% error or better
- **Reasonable tau**: Visible in debug output (typically 1-6 hours for BBQ)
- **Stable velocity**: Not wildly fluctuating between measurements

## Advanced Customization

### Custom Grill Profiles
You can create cooking-style-specific profiles by modifying the grill dynamics factors:

```go
// Competition BBQ profile (conservative)
grillFactors := []float64{0.05, 0.1, 0.3, 0.2}

// Fast grilling profile (aggressive)  
grillFactors := []float64{0.2, 0.4, 0.7, 0.5}
```

### Environmental Factors
Consider adding parameters for:
- Ambient temperature effects
- Wind/weather conditions
- Meat wrapping (Texas crutch)
- Probe placement (thickness dependency)

The exponential prediction system provides a solid physics-based foundation that can be tuned for your specific cooking environment and style.
