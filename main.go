package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fogleman/primitive/internal/logger"
	"github.com/fogleman/primitive/primitive"
	"github.com/nfnt/resize"
)

var (
	Input      string
	Outputs    flagArray
	Background string
	Configs    shapeConfigArray
	Alpha      int
	InputSize  int
	OutputSize int
	Mode       int
	Workers    int
	Nth        int
	Repeat     int
	V, VV      bool
)

type flagArray []string

func (i *flagArray) String() string {
	return strings.Join(*i, ", ")
}

func (i *flagArray) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type shapeConfig struct {
	Count  int
	Mode   int
	Alpha  int
	Repeat int
}

type shapeConfigArray []shapeConfig

func (i *shapeConfigArray) String() string {
	return ""
}

func (i *shapeConfigArray) Set(value string) error {
	n, err := strconv.ParseInt(value, 0, 0)
	if err != nil {
		return err
	}
	*i = append(*i, shapeConfig{int(n), Mode, Alpha, Repeat})
	return nil
}

func init() {
	flag.StringVar(&Input, "i", "", "input image path")
	flag.Var(&Outputs, "o", "output image path")
	flag.Var(&Configs, "n", "number of primitives")
	flag.StringVar(&Background, "bg", "", "background color (hex)")
	flag.IntVar(&Alpha, "a", 128, "alpha value")
	flag.IntVar(&InputSize, "r", 256, "resize large input images to this size")
	flag.IntVar(&OutputSize, "s", 1024, "output image size")
	flag.IntVar(&Mode, "m", 1, "0=combo 1=triangle 2=rect 3=ellipse 4=circle 5=rotatedrect 6=beziers 7=rotatedellipse 8=polygon")
	flag.IntVar(&Workers, "j", 0, "number of parallel workers (default uses all cores)")
	flag.IntVar(&Nth, "nth", 1, "save every Nth frame (put \"%d\" in path)")
	flag.IntVar(&Repeat, "rep", 0, "add N extra shapes per iteration with reduced search")
	flag.BoolVar(&V, "v", false, "verbose")
	flag.BoolVar(&VV, "vv", false, "very verbose")
}

func main() {
	os.Exit(run())
}

func run() int {
	ctx := context.Background()
	if err := doRun(ctx); err != nil {
		if hasExitCode, ok := err.(interface{ ExitCode() int }); ok {
			return hasExitCode.ExitCode()
		}
		return 1
	}
	return 0
}

func doRun(ctx context.Context) error {
	flag.Parse()
	var err error
	if Input == "" {
		err = errors.Join(err, errors.New("ERROR: input argument required"))
	}
	if len(Outputs) == 0 {
		err = errors.Join(err, errors.New("ERROR: output argument required"))
	}
	if len(Configs) == 0 {
		err = errors.Join(err, errors.New("ERROR: number argument required"))
	}
	if len(Configs) == 1 {
		Configs[0].Mode = Mode
		Configs[0].Alpha = Alpha
		Configs[0].Repeat = Repeat
	}
	for _, config := range Configs {
		if config.Count < 1 {
			err = errors.Join(err, errors.New("ERROR: number argument must be > 0"))
		}
	}
	if err != nil {
		return err
	}

	logLevel := slog.LevelWarn
	// set log level
	if V {
		logLevel = slog.LevelInfo
	}
	if VV {
		logLevel = slog.LevelDebug
	}
	logger.SetupLogger(os.Stderr, logLevel)

	// determine worker count
	if Workers < 1 {
		Workers = runtime.NumCPU()
	}

	// read input image
	slog.DebugContext(ctx, "reading input", slog.String("input", Input))
	input, err := primitive.LoadImage(Input)
	if err != nil {
		return err
	}

	// scale down input image if needed
	size := uint(InputSize)
	if size > 0 {
		input = resize.Thumbnail(size, size, input, resize.Bilinear)
	}

	// determine background color
	var bg primitive.Color
	if Background == "" {
		bg = primitive.MakeColor(primitive.AverageImageColor(input))
	} else {
		bg = primitive.MakeHexColor(Background)
	}

	// run algorithm
	model := primitive.NewModel(input, bg, OutputSize, Workers)
	slog.InfoContext(ctx, "run algorithm",
		slog.Int("frame", 0),
		slog.Float64("t", 0.0),
		slog.Float64("score", model.Score),
	)
	start := time.Now()
	frame := 0
	for j, config := range Configs {
		slog.InfoContext(ctx, "", slog.Int("count", config.Count), slog.Int("mode", config.Mode), slog.Int("alpha", config.Alpha), slog.Int("repeat", config.Repeat))

		for i := 0; i < config.Count; i++ {
			frame++

			// find optimal shape and add it to the model
			t := time.Now()
			n := model.Step(primitive.ShapeType(config.Mode), config.Alpha, config.Repeat)
			nps := primitive.NumberString(float64(n) / time.Since(t).Seconds())
			elapsed := time.Since(start).Seconds()
			slog.InfoContext(ctx, "",
				slog.Int("frame", frame),
				slog.Float64("t", elapsed),
				slog.Float64("score", model.Score),
				slog.Int("n", n),
				slog.String("nps", nps),
			)

			// write output image(s)
			for _, output := range Outputs {
				ext := strings.ToLower(filepath.Ext(output))
				if output == "-" {
					ext = ".svg"
				}
				percent := strings.Contains(output, "%")
				saveFrames := percent && ext != ".gif"
				saveFrames = saveFrames && frame%Nth == 0
				last := j == len(Configs)-1 && i == config.Count-1
				if saveFrames || last {
					path := output
					if percent {
						path = fmt.Sprintf(output, frame)
					}
					slog.InfoContext(ctx, "writing", slog.String("output", path))
					switch ext {
					default:
						return fmt.Errorf("unrecognized file extension: %s", ext)
					case ".png":
						return primitive.SavePNG(path, model.Context.Image())
					case ".jpg", ".jpeg":
						return primitive.SaveJPG(path, model.Context.Image(), 95)
					case ".svg":
						return primitive.SaveFile(path, model.SVG())
					case ".gif":
						frames := model.Frames(0.001)
						return primitive.SaveGIFImageMagick(ctx, path, frames, 50, 250)
					}
				}
			}
		}
	}
	return nil
}
