package main

import (
	"bufio"
	"encoding/json"
	"image/color"
	"os"
	"time"

	"github.com/spf13/cobra"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"

	"github.com/endobit/wifire"
)

type tempdata struct {
	time     float64
	ambient  int
	grill    int
	probe    int
	grillSet int
	probeSet int
}

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

			var (
				temps []tempdata
				t0    time.Time
			)

			for s := bufio.NewScanner(fin); s.Scan(); {
				var status wifire.Status

				if err := json.Unmarshal(s.Bytes(), &status); err != nil {
					return err
				}

				if t0.IsZero() {
					t0 = status.Time
				}

				temps = append(temps, tempdata{
					time:     status.Time.Sub(t0).Hours(),
					ambient:  status.Ambient,
					grill:    status.Grill,
					probe:    status.Probe,
					grillSet: status.GrillSet,
					probeSet: status.ProbeSet,
				})
			}

			hours := make([]float64, len(markers))
			for i, m := range markers {
				hours[i] = m.Hours()
			}
			return scatter(t0.Format(time.ANSIC), output, temps, hours)
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

// Implement XYers for each temperature, this is easiers than building an XYs
// for each.
type (
	temps        []tempdata
	ambientData  struct{ temps }
	grillData    struct{ temps }
	probeData    struct{ temps }
	grillSetData struct{ temps }
	probeSetData struct{ temps }
)

func (t temps) Len() int {
	return len(t)
}

func (a ambientData) XY(i int) (x, y float64) {
	return a.temps[i].time, float64(a.temps[i].ambient)
}

func (g grillData) XY(i int) (x, y float64) {
	return g.temps[i].time, float64(g.temps[i].grill)
}

func (p probeData) XY(i int) (x, y float64) {
	return p.temps[i].time, float64(p.temps[i].probe)
}

func (g grillSetData) XY(i int) (x, y float64) {
	return g.temps[i].time, float64(g.temps[i].grillSet)
}

func (g grillSetData) Max() float64 {
	var max float64

	for i := 0; i < g.Len(); i++ {
		_, y := g.XY(i)
		if y > max {
			max = y
		}
	}

	return max
}

func (p probeSetData) XY(i int) (x, y float64) {
	return p.temps[i].time, float64(p.temps[i].probeSet)
}

func scatter(title, filename string, data []tempdata, markers []float64) error {
	p := plot.New()

	p.Title.Text = title
	p.X.Label.Text = "Hours"
	p.Y.Label.Text = "Temperature"

	ambient := ambientData{data}
	grill := grillData{data}
	probe := probeData{data}
	grillSet := grillSetData{data}
	probeSet := probeSetData{data}

	sa, err := plotter.NewLine(ambient)
	if err != nil {
		return err
	}

	sg, err := plotter.NewLine(grill)
	if err != nil {
		return err
	}

	sp, err := plotter.NewLine(probe)
	if err != nil {
		return err
	}

	sgs, err := plotter.NewLine(grillSet)
	if err != nil {
		return err
	}

	sps, err := plotter.NewLine(probeSet)
	if err != nil {
		return err
	}

	marks := make(plotter.XYs, len(markers))
	for i, x := range markers {
		marks[i].X = x
		marks[i].Y = grillSet.Max() / 2
	}
	m, err := plotter.NewScatter(marks)
	if err != nil {
		return err
	}

	m.GlyphStyle.Shape = draw.CrossGlyph{}
	m.GlyphStyle.Radius = vg.Points(4)
	m.Color = color.RGBA{G: 100, A: 255}

	sa.Color = color.Gray{200}
	sa.FillColor = color.Gray{200}

	sg.Color = color.RGBA{R: 255, A: 255}
	sgs.LineStyle.Dashes = []vg.Length{vg.Points(1), vg.Points(5)}
	sgs.Color = sg.Color

	sp.Color = color.RGBA{B: 255, A: 255}
	sps.LineStyle.Dashes = []vg.Length{vg.Points(1), vg.Points(5)}
	sps.Color = sp.Color

	p.Add(plotter.NewGrid(), sa, sg, sp, sgs, sps, m)
	p.Legend.Add("air", sa)
	p.Legend.Add("grill", sg)
	p.Legend.Add("probe", sp)
	p.Legend.Add("events", m)

	if err := p.Save(800, 300, filename); err != nil {
		return err
	}

	return nil
}
