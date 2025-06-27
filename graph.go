package wifire

import (
	"errors"
	"fmt"
	"image/color"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// PlotterOptions is used to configure the Plotter.
type PlotterOptions struct {
	Title            string
	Period           Period
	AmbientColor     color.Color
	AmbientFillColor color.Color
	ProbeColor       color.Color
	GrillColor       color.Color
	MarkerColor      color.Color
	Data             []Status
	Markers          []time.Duration
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
	if p.options.Data == nil {
		return nil, errors.New("no data")
	}

	ambient := make(plotter.XYs, len(p.options.Data))

	for i, d := range normalizeStatus(p.options.Data) {
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
	grillSet := make(plotter.XYs, len(ambient))
	probeSet := make(plotter.XYs, len(ambient))

	copy(grill, ambient)
	copy(probe, ambient)
	copy(grillSet, ambient)
	copy(probeSet, ambient)

	var maxTemp int

	for i := range p.options.Data {
		if p.options.Data[i].Grill > maxTemp {
			maxTemp = p.options.Data[i].Grill
		}

		grill[i].Y = float64(p.options.Data[i].Grill)
		probe[i].Y = float64(p.options.Data[i].Probe)
		grillSet[i].Y = float64(p.options.Data[i].Grill)
		probeSet[i].Y = float64(p.options.Data[i].Probe)
	}

	markers := make(plotter.XYs, len(p.options.Markers))

	for i, m := range p.options.Markers {
		switch p.options.Period {
		case ByMinute:
			markers[i].X = m.Minutes()
		case ByHour:
			markers[i].X = m.Hours()
		case ByDay:
			markers[i].X = m.Hours() / 24
		}

		markers[i].Y = float64(maxTemp) / 2 // put markers in the middle of the data
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

	if len(markers) > 0 {
		if err := p.markers(markers); err != nil {
			return nil, fmt.Errorf("markers: %w", err)
		}
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

func (p *Plotter) markers(marks plotter.XYs) error {
	if marks == nil {
		return nil // markers are optional
	}

	m, err := plotter.NewScatter(marks)
	if err != nil {
		return err
	}

	m.Shape = draw.CrossGlyph{}
	m.Radius = vg.Points(4)
	m.Color = p.options.MarkerColor
	p.plot.Add(m)
	p.plot.Legend.Add("events", m)

	return nil
}

func normalizeStatus(s []Status) []time.Duration {
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
