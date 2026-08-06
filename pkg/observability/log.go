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
	return slog.New(slog.NewJSONHandler(output, nil)).With("service", service)
}
