package cmd

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/honeok/homedash-core/internal/config"
	"github.com/honeok/homedash-core/internal/media"
	"github.com/honeok/homedash-core/internal/status"
	"github.com/honeok/homedash-core/internal/weather"
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
	mux.Handle("GET /v1/status", status.NewHandler())
	mux.Handle("GET /v1/weather", weatherHandler)
	mux.Handle("GET /v1/media", media.NewHandler(cfg.Media))

	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           accessLog(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("Starting HTTP server", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}
	return nil
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(data)
}

// 记录 HTTP 请求
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		response := &statusResponseWriter{ResponseWriter: w}

		next.ServeHTTP(response, r)
		if response.status == 0 {
			response.status = http.StatusOK
		}
		slog.Info("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", response.status,
			"duration", time.Since(startedAt),
		)
	})
}
