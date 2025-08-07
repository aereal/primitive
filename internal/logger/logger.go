package logger

import (
	"fmt"
	"io"
	"log/slog"
)

func SetupLogger(out io.Writer, logLevel slog.Level) {
	opts := &slog.HandlerOptions{
		AddSource: false,
		Level:     logLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Value.Kind() {
			case slog.KindFloat64:
				return slog.Attr{
					Key:   a.Key,
					Value: slog.StringValue(fmt.Sprintf("%.6f", a.Value.Float64())),
				}
			default:
				return a
			}
		},
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(out, opts)))
}
