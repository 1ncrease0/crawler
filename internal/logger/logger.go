package logger

import (
	"fmt"
	"log/slog"
	"os"
)

func Setup(path string) (*slog.Logger, func() error, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file %q: %w", path, err)
	}

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(handler), file.Close, nil
}
