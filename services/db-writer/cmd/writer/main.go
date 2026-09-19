package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"db-writer/internal/metrics"
	"db-writer/internal/writer"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	postgresURL := os.Getenv("POSTGRES_URL")
	if postgresURL == "" {
		postgresURL = "postgres://carestack:cs_dev_password@localhost:5432/carestack_charting?sslmode=disable"
	}

	// Connect Redis
	rOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error("Invalid Redis URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(rOpts)

	// Connect PostgreSQL
	db, err := sql.Open("postgres", postgresURL)
	if err != nil {
		logger.Error("Failed to open PostgreSQL", "error", err)
		os.Exit(1)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	latencyRecorder := metrics.NewRecorder(500)
	consumer := writer.NewConsumer(rdb, db, logger, latencyRecorder)

	// ── Metrics HTTP server ─────────────────────────────────────
	// db-writer has no other HTTP surface today — this is purely for
	// /metrics/latency so queue_wait / db_write / end_to_end can be
	// checked from a browser or dashboard.
	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9091"
	}
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics/latency", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"service": "db-writer",
			"stages":  latencyRecorder.Snapshot(),
		})
	})
	metricsSrv := &http.Server{Addr: ":" + metricsPort, Handler: metricsMux}
	go func() {
		logger.Info("DB-Writer metrics server started", "port", metricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Metrics server error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("🚀 DB-Writer service started")
		if err := consumer.Run(); err != nil {
			logger.Error("Consumer error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	logger.Info("DB-Writer shutting down")
	consumer.Stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = metricsSrv.Shutdown(shutdownCtx)
}
