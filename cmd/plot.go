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

	"github.com/endobit/wifire.git"
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
		input  string
		output string
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

			return scatter(t0.Format(time.ANSIC), output, temps)
		},
	}

	cmd.Flags().StringVarP(&input, "input", "i", "", "input file")
	cmd.Flags().StringVarP(&output, "output", "o", "wifire.png", "output file")

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

func (p probeSetData) XY(i int) (x, y float64) {
	return p.temps[i].time, float64(p.temps[i].probeSet)
}

func scatter(title, filename string, data []tempdata) error {
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

	sa.Color = color.Gray{200}
	sa.FillColor = color.Gray{200}

	sg.Color = color.RGBA{R: 255, A: 255}
	sgs.LineStyle.Dashes = []vg.Length{vg.Points(1), vg.Points(5)}
	sgs.Color = sg.Color

	sp.Color = color.RGBA{B: 255, A: 255}
	sps.LineStyle.Dashes = []vg.Length{vg.Points(1), vg.Points(5)}
	sps.Color = sp.Color

	p.Add(plotter.NewGrid(), sa, sg, sp, sgs, sps)
	p.Legend.Add("air", sa)
	p.Legend.Add("grill", sg)
	p.Legend.Add("probe", sp)

	if err := p.Save(800, 300, filename); err != nil {
		return err
	}

	return nil
}
