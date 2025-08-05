package primitive

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/fogleman/gg"
)

type Model struct {
	Target     *image.RGBA
	Current    *image.RGBA
	Context    *gg.Context
	Shapes     []Shape
	Colors     []Color
	Scores     []float64
	Workers    []*Worker
	Background Color
	Sw         int
	Sh         int
	Scale      float64
	Score      float64
}

func NewModel(target image.Image, background Color, size, numWorkers int) *Model {
	w := target.Bounds().Size().X
	h := target.Bounds().Size().Y
	aspect := float64(w) / float64(h)
	var sw, sh int
	var scale float64
	if aspect >= 1 {
		sw = size
		sh = int(float64(size) / aspect)
		scale = float64(size) / float64(w)
	} else {
		sw = int(float64(size) * aspect)
		sh = size
		scale = float64(size) / float64(h)
	}

	targetRGBA := imageToRGBA(target)
	current := uniformRGBA(target.Bounds(), background.NRGBA())
	model := &Model{
		Sw:         sw,
		Sh:         sh,
		Scale:      scale,
		Background: background,
		Target:     targetRGBA,
		Current:    current,
		Score:      differenceFull(targetRGBA, current),
		Context:    newModelContext(sw, sh, scale, background.NRGBA()),
		Shapes:     nil,
		Colors:     nil,
		Scores:     nil,
		Workers:    nil,
	}
	for i := 0; i < numWorkers; i++ {
		worker := NewWorker(model.Target)
		model.Workers = append(model.Workers, worker)
	}
	return model
}

func newModelContext(sw, sh int, scale float64, color color.NRGBA) *gg.Context {
	dc := gg.NewContext(sw, sh)
	dc.Scale(scale, scale)
	dc.Translate(0.5, 0.5)
	dc.SetColor(color)
	dc.Clear()
	return dc
}

func (model *Model) Frames(scoreDelta float64) []image.Image {
	var result []image.Image
	dc := newModelContext(model.Sw, model.Sh, model.Scale, model.Background.NRGBA())
	result = append(result, imageToRGBA(dc.Image()))
	previous := 10.0
	for i, shape := range model.Shapes {
		c := model.Colors[i]
		dc.SetRGBA255(c.R, c.G, c.B, c.A)
		shape.Draw(dc, model.Scale)
		dc.Fill()
		score := model.Scores[i]
		delta := previous - score
		if delta >= scoreDelta {
			previous = score
			result = append(result, imageToRGBA(dc.Image()))
		}
	}
	return result
}

func (model *Model) SVG() string {
	bg := model.Background
	b := new(strings.Builder)
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="%d" height="%d">`, model.Sw, model.Sh)
	fmt.Fprintln(b)
	fmt.Fprintf(b, `<rect x="0" y="0" width="%d" height="%d" fill="#%02x%02x%02x" />`, model.Sw, model.Sh, bg.R, bg.G, bg.B)
	fmt.Fprintln(b)
	fmt.Fprintf(b, `<g transform="scale(%f) translate(0.5 0.5)">`, model.Scale)
	fmt.Fprintln(b)
	for i, shape := range model.Shapes {
		c := model.Colors[i]
		attrs := fmt.Sprintf(`fill="#%02x%02x%02x" fill-opacity="%f"`, c.R, c.G, c.B, float64(c.A)/255)
		fmt.Fprint(b, shape.SVG(attrs))
		fmt.Fprintln(b)
	}
	fmt.Fprintln(b, "</g>")
	fmt.Fprintln(b, "</svg>")
	return b.String()
}

func (model *Model) Add(shape Shape, alpha int) {
	before := copyRGBA(model.Current)
	lines := shape.Rasterize()
	color := computeColor(model.Target, model.Current, lines, alpha)
	drawLines(model.Current, color, lines)
	score := differencePartial(model.Target, before, model.Current, model.Score, lines)

	model.Score = score
	model.Shapes = append(model.Shapes, shape)
	model.Colors = append(model.Colors, color)
	model.Scores = append(model.Scores, score)

	model.Context.SetRGBA255(color.R, color.G, color.B, color.A)
	shape.Draw(model.Context, model.Scale)
}

func (model *Model) Step(ctx context.Context, shapeType ShapeType, alpha, repeat int) int {
	state := model.runWorkers(ctx, shapeType, alpha, 1000, 100, 16)
	// state = HillClimb(state, 1000).(*State)
	model.Add(state.Shape, state.Alpha)

	for range repeat {
		state.Worker.Init(model.Current, model.Score)
		a := state.Energy()
		state, _ = HillClimb(state, 100).(*State)
		b := state.Energy()
		if a == b {
			break
		}
		model.Add(state.Shape, state.Alpha)
	}

	// for _, w := range model.Workers[1:] {
	// 	model.Workers[0].Heatmap.AddHeatmap(w.Heatmap)
	// }
	// SavePNG("heatmap.png", model.Workers[0].Heatmap.Image(0.5))

	counter := 0
	for _, worker := range model.Workers {
		counter += worker.Counter
	}
	return counter
}

func (model *Model) runWorkers(ctx context.Context, t ShapeType, a, n, age, m int) *State {
	wn := len(model.Workers)
	ch := make(chan *State, wn)
	wm := m / wn
	if m%wn != 0 {
		wm++
	}
	for i := range wn {
		worker := model.Workers[i]
		worker.Init(model.Current, model.Score)
		go model.runWorker(ctx, worker, t, a, n, age, wm, ch)
	}
	var bestEnergy float64
	var bestState *State
	for i := range wn {
		state := <-ch
		energy := state.Energy()
		if i == 0 || energy < bestEnergy {
			bestEnergy = energy
			bestState = state
		}
	}
	return bestState
}

func (model *Model) runWorker(ctx context.Context, worker *Worker, t ShapeType, a, n, age, m int, ch chan *State) {
	ch <- worker.BestHillClimbState(ctx, t, a, n, age, m)
}
