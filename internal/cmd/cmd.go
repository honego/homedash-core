package cmd

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/honeok/homepage-core/internal/config"
	"github.com/honeok/homepage-core/internal/core"
	"github.com/honeok/homepage-core/internal/weather"
)

func Run() error {
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	configPath := flags.String("c", "config.yaml", "Path to the configuration file.")
	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("failed to parse command-line flags: %w", err)
	}

	slog.Info("Loading configuration", "file", *configPath)
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	weatherHandler, err := weather.NewHandler(cfg.Weather)
	if err != nil {
		return fmt.Errorf("failed to initialize weather handler: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /v1/runtime", core.NewRuntimeHandler())
	mux.Handle("/v1/weather", weatherHandler)

	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("Starting HTTP server", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}
