package observability

import (
	"io"
	"log/slog"
	"os"
)

func NewLogger(service string, output io.Writer) *slog.Logger {
	if output == nil {
		output = os.Stdout
	}
	options := &slog.HandlerOptions{ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
		switch attr.Key {
		case slog.TimeKey:
			attr.Key = "timestamp"
		case slog.MessageKey:
			attr.Key = "message"
		}
		return attr
	}}
	return slog.New(slog.NewJSONHandler(output, options)).With("service", service)
}
