package main

import (
	"log/slog"
	"os"

	"github.com/honeok/homedash-core/internal/cmd"
	"github.com/honeok/homedash-core/internal/core"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	slog.Info(core.VersionStatement())

	if err := cmd.Run(); err != nil {
		slog.Error("Application exited with error", "err", err)
		os.Exit(1)
	}
}
