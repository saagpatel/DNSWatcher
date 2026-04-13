package main

import (
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"dnswatcher/backend/internal/httpapi"
	"dnswatcher/backend/internal/trace"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	engine := trace.NewEngine(trace.Config{})
	staticDir := os.Getenv("DNSWATCHER_STATIC_DIR")
	if staticDir == "" {
		staticDir = filepath.Clean("../frontend/dist")
		if _, err := os.Stat(staticDir); err != nil {
			staticDir = ""
		}
	}
	server := httpapi.NewServer(engine, httpapi.Config{
		Logger:              logger,
		BodyLimitBytes:      envInt64("DNSWATCHER_BODY_LIMIT_BYTES", 2048),
		RateLimitPerMinute:  envInt("DNSWATCHER_RATE_LIMIT_PER_MINUTE", 20),
		Burst:               envInt("DNSWATCHER_RATE_LIMIT_BURST", 5),
		MaxConcurrentTraces: envInt("DNSWATCHER_MAX_CONCURRENT_TRACES", 8),
		ReadHeaderTimeout:   envDuration("DNSWATCHER_READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:         envDuration("DNSWATCHER_READ_TIMEOUT", 10*time.Second),
		WriteTimeout:        envDuration("DNSWATCHER_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:         envDuration("DNSWATCHER_IDLE_TIMEOUT", 60*time.Second),
		StaticDir:           staticDir,
	})
	httpServer := httpapi.StdlibServer(server.Handler(), httpapi.Config{
		ReadHeaderTimeout: envDuration("DNSWATCHER_READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:       envDuration("DNSWATCHER_READ_TIMEOUT", 10*time.Second),
		WriteTimeout:      envDuration("DNSWATCHER_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:       envDuration("DNSWATCHER_IDLE_TIMEOUT", 60*time.Second),
		MaxHeaderBytes:    envInt("DNSWATCHER_MAX_HEADER_BYTES", 1<<20),
	})
	httpServer.Addr = net.JoinHostPort(envString("HOST", ""), envString("PORT", "8080"))
	log.Printf("dnswatcher api listening on %s", httpServer.Addr)
	log.Fatal(httpServer.ListenAndServe())
}

func envString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
