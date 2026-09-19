package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"voice-session/internal/groq"
	"voice-session/internal/metrics"
	"voice-session/internal/queue"
	"voice-session/internal/session"
	"voice-session/internal/state"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "dev-jwt-secret-change-in-prod"
	}

	deepgramAPIKey := os.Getenv("DEEPGRAM_API_KEY")
	mockMode := deepgramAPIKey == ""
	if mockMode {
		logger.Info("⚠  DEEPGRAM_API_KEY not set — starting in Demo/Mock mode")
	}

	groqAPIKey := os.Getenv("GROQ_API_KEY")
	groqClient := groq.NewClient(groqAPIKey)
	if groqClient.Enabled() {
		logger.Info("✅ Groq AI slow-path enabled", "model", "llama3-8b-8192")
	} else {
		logger.Info("⚠  GROQ_API_KEY not set — Groq slow-path disabled (regex only)")
	}

	// ── Redis state store ───────────────────────────────────────
	redisStore, err := state.NewStore(redisURL)
	if err != nil {
		logger.Error("Failed to connect to Redis", "url", redisURL, "error", err)
		os.Exit(1)
	}
	logger.Info("Connected to Redis", "url", redisURL)

	// ── Redis Streams queue producer ────────────────────────────
	queueProducer := queue.NewProducer(redisStore.Client())

	// ── Session manager ─────────────────────────────────────────
	mgr := session.NewManager(redisStore, queueProducer, logger)

	// ── Latency instrumentation ──────────────────────────────────
	// Rolling window of the last 500 samples per pipeline stage.
	// See /metrics/latency below for how to read it.
	latencyRecorder := metrics.NewRecorder(500)

	// ── WebSocket handler ───────────────────────────────────────
	wsHandler := session.NewHandler(mgr, redisStore, queueProducer, groqClient, jwtSecret, mockMode, logger, latencyRecorder)

	// ── HTTP routes ─────────────────────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":       "ok",
			"mock_mode":    mockMode,
			"groq_enabled": groqClient.Enabled(),
			"sessions":     mgr.Count(),
			"service":      "voice-session",
			"version":      "2.0.0",
		})
	})

	mux.Handle("/ws/stream", wsHandler)

	// ── Latency metrics ──────────────────────────────────────────
	// Real, measured latency per pipeline stage (not an estimate):
	// parse_regex / parse_groq — time spent turning a transcript into a
	//   structured ChartEvent
	// speech_to_chart          — transcript received → chart_event pushed
	//   to the browser (this is the number that matters for "does it feel
	//   instant" — everything after this point is async/off the hot path)
	// ws_push                  — time to write the chart_event frame to
	//   the open WebSocket
	// redis_enqueue            — time to hand the event to the async
	//   persistence queue (should be near-zero; if it isn't, the hot
	//   path is being slowed down by Redis and that's worth knowing)
	//
	// CORS is opened here deliberately: this is read-only, non-sensitive
	// latency data, meant to be pollable from a dashboard page hosted
	// anywhere (including a page that isn't served by this app).
	mux.HandleFunc("/metrics/latency", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"service": "voice-session",
			"stages":  latencyRecorder.Snapshot(),
		})
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("🚀 Voice session service started", "port", port, "mock_mode", mockMode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Forced shutdown", "error", err)
	}
	logger.Info("Server stopped cleanly")
}
