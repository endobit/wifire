package wifire

import (
	"errors"
	"fmt"
	"image/color"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

// PlotterOptions is used to configure the Plotter.
type PlotterOptions struct {
	Title            string
	Period           Period
	AmbientColor     color.Color
	AmbientFillColor color.Color
	ProbeColor       color.Color
	ProbeETAColor    color.Color
	GrillColor       color.Color
	MarkerColor      color.Color
	Data             []Status
	Markers          []Marker
}

// Marker is an annotation point on the graph.
type Marker struct {
	Label string
	Time  time.Time
}

// Plotter creates a graph of the wifire Status data.
type Plotter struct {
	options PlotterOptions
	plot    *plot.Plot
}

// Period is used to set the x-axis time period.
type Period int

// The Period can be hours, minutes, or days. The default is hours.
const (
	ByHour Period = iota
	ByMinute
	ByDay
)

// NewPlotter returns a Plotter configured with the options o. If o is empty the
// default settings are used.
func NewPlotter(options *PlotterOptions) *Plotter {
	p := Plotter{ //nolint:varnamelen
		options: PlotterOptions{
			AmbientColor:     color.Gray{200},
			AmbientFillColor: color.Gray{200},
			ProbeColor:       color.RGBA{B: 255, A: 255},
			ProbeETAColor:    color.RGBA{R: 255, G: 165, A: 255}, // Orange for ETA
			GrillColor:       color.RGBA{R: 255, A: 255},
			MarkerColor:      color.RGBA{G: 100, A: 255},
		},
	}

	p.options.Title = options.Title
	p.options.Period = options.Period
	p.options.Data = options.Data
	p.options.Markers = options.Markers

	if options.AmbientColor != nil {
		p.options.AmbientColor = options.AmbientColor
	}

	if options.AmbientFillColor != nil {
		p.options.AmbientFillColor = options.AmbientFillColor
	}

	if options.ProbeColor != nil {
		p.options.ProbeColor = options.ProbeColor
	}

	if options.ProbeETAColor != nil {
		p.options.ProbeETAColor = options.ProbeETAColor
	}

	if options.GrillColor != nil {
		p.options.GrillColor = options.GrillColor
	}

	if options.AmbientColor != nil {
		p.options.MarkerColor = options.MarkerColor
	}

	return &p
}

// Plot returns the plot.Plot for the Status data given to the Plotter. The
// caller should call plot.Save to create the graph files. This allows the
// caller to define the Plot size and graphics format.
func (p *Plotter) Plot() (*plot.Plot, error) {
	duration := statusDuration(p.options.Data)
	if duration == nil {
		return nil, errors.New("no data")
	}

	ambient := make(plotter.XYs, len(p.options.Data))

	for i, d := range duration {
		switch p.options.Period {
		case ByMinute:
			ambient[i].X = d.Minutes()
		case ByHour:
			ambient[i].X = d.Hours()
		case ByDay:
			ambient[i].X = d.Hours() / 24
		}

		ambient[i].Y = float64(p.options.Data[i].Ambient)
	}

	grill := make(plotter.XYs, len(ambient))
	probe := make(plotter.XYs, len(ambient))
	probeETA := make(plotter.XYs, 0) // Variable length as not all points have ETA
	grillSet := make(plotter.XYs, len(ambient))
	probeSet := make(plotter.XYs, len(ambient))

	copy(grill, ambient)
	copy(probe, ambient)
	copy(grillSet, ambient)
	copy(probeSet, ambient)

	var (
		maxTemp int
		maxETA  float64
	)

	for i := range p.options.Data {
		if p.options.Data[i].Grill > maxTemp {
			maxTemp = p.options.Data[i].Grill
		}

		grill[i].Y = float64(p.options.Data[i].Grill)
		probe[i].Y = float64(p.options.Data[i].Probe)
		grillSet[i].Y = float64(p.options.Data[i].Grill)
		probeSet[i].Y = float64(p.options.Data[i].Probe)

		// Add probeETA data points only where ETA exists
		if p.options.Data[i].ProbeETA > 0 {
			var xValue float64

			switch p.options.Period {
			case ByMinute:
				xValue = duration[i].Minutes()
			case ByHour:
				xValue = duration[i].Hours()
			case ByDay:
				xValue = duration[i].Hours() / 24
			}

			etaMinutes := p.options.Data[i].ProbeETA.Minutes()
			if etaMinutes > maxETA {
				maxETA = etaMinutes
			}

			probeETA = append(probeETA, plotter.XY{
				X: xValue,
				Y: etaMinutes,
			})
		}
	}

	var (
		markerXYs    plotter.XYs
		markerLabels []string
	)

	t0 := p.options.Data[0].Time

	for _, m := range p.options.Markers {
		var xValue float64

		d := m.Time.Sub(t0)

		switch p.options.Period {
		case ByMinute:
			xValue = d.Minutes()
		case ByHour:
			xValue = d.Hours()
		case ByDay:
			xValue = d.Hours() / 24
		}

		markerXYs = append(markerXYs, plotter.XY{
			X: xValue,
			Y: float64(maxTemp) / 2, // put markers in the middle of the data
		})

		markerLabels = append(markerLabels, m.Label)
	}

	markers, err := plotter.NewLabels(plotter.XYLabels{
		XYs:    markerXYs,
		Labels: markerLabels,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create markers: %w", err)
	}

	for i := range markers.TextStyle {
		markers.TextStyle[i].Color = p.options.MarkerColor
	}

	p.plot = plot.New()
	p.plot.Title.Text = p.options.Title
	p.plot.X.Label.Text = "Hours"
	p.plot.Y.Label.Text = "Temperature"

	if err := p.ambient(ambient); err != nil {
		return nil, fmt.Errorf("ambient: %w", err)
	}

	if err := p.grill(grill, grillSet); err != nil {
		return nil, fmt.Errorf("grill: %w", err)
	}

	if err := p.probe(probe, probeSet); err != nil {
		return nil, fmt.Errorf("probe: %w", err)
	}

	if len(probeETA) > 0 {
		if err := p.probeETA(probeETA, maxTemp, maxETA); err != nil {
			return nil, fmt.Errorf("probe ETA: %w", err)
		}
	}

	if len(markers.Labels) > 0 {
		p.plot.Add(markers)
	}

	p.plot.Add(plotter.NewGrid())

	return p.plot, nil
}

func (p *Plotter) ambient(data plotter.XYs) error {
	if data == nil {
		return errors.New("no ambient data")
	}

	line, err := plotter.NewLine(data)
	if err != nil {
		return err
	}

	line.Color = p.options.AmbientColor
	line.FillColor = p.options.AmbientFillColor
	p.plot.Add(line)
	p.plot.Legend.Add("ambient", line)

	return nil
}

func (p *Plotter) grill(actual, set plotter.XYs) error {
	if actual == nil {
		return errors.New("no grill data")
	}

	a, err := plotter.NewLine(actual)
	if err != nil {
		return err
	}

	a.Color = p.options.GrillColor
	p.plot.Add(a)
	p.plot.Legend.Add("grill", a)

	if set == nil {
		return nil
	}

	s, err := plotter.NewLine(set)
	if err != nil {
		return err
	}

	s.Color = p.options.GrillColor
	s.Dashes = []vg.Length{vg.Points(1), vg.Points(5)}
	p.plot.Add(s)

	return nil
}

func (p *Plotter) probe(actual, set plotter.XYs) error {
	if actual == nil {
		return errors.New("no probe data")
	}

	a, err := plotter.NewLine(actual)
	if err != nil {
		return err
	}

	a.Color = p.options.ProbeColor
	p.plot.Add(a)
	p.plot.Legend.Add("probe", a)

	if set == nil {
		return nil
	}

	s, err := plotter.NewLine(set)
	if err != nil {
		return err
	}

	s.Color = p.options.ProbeColor
	s.Dashes = []vg.Length{vg.Points(1), vg.Points(5)}
	p.plot.Add(s)

	return nil
}

func (p *Plotter) probeETA(data plotter.XYs, maxTemp int, maxETA float64) error {
	if len(data) == 0 {
		return nil // probeETA is optional
	}

	// Scale ETA to fit within the temperature range for visibility
	// We'll scale it to use the upper portion of the temperature range
	scaleFactor := float64(maxTemp) * 0.3 / maxETA // Use 30% of temp range for ETA

	// Scale all Y values
	for i := range data {
		data[i].Y *= scaleFactor
		data[i].Y += float64(maxTemp) * 0.7 // Position in upper 30% of chart
	}

	// Create a line plot for the ETA data
	line, err := plotter.NewLine(data)
	if err != nil {
		return err
	}

	line.Color = p.options.ProbeETAColor
	line.Dashes = []vg.Length{vg.Points(2), vg.Points(3)} // Dotted line to distinguish from other data

	p.plot.Add(line)
	p.plot.Legend.Add("probe ETA (scaled)", line)

	return nil
}

func statusDuration(s []Status) []time.Duration {
	if len(s) == 0 {
		return nil
	}

	d := make([]time.Duration, len(s))

	t0 := s[0].Time
	for i := range s {
		d[i] = s[i].Time.Sub(t0)
	}

	return d
}
