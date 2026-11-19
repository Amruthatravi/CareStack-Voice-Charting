package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"perioassist/internal/api"
	"perioassist/internal/config"
	"perioassist/internal/llm"
	"perioassist/internal/service"
	"perioassist/internal/store"
	"perioassist/internal/stream"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()
	for i := 0; ; i++ {
		if err := rdb.Ping(ctx).Err(); err == nil {
			break
		} else if i >= 30 {
			log.Fatalf("redis: %v", err)
		}
		time.Sleep(time.Second)
	}

	l := llm.New(llm.Config{
		APIKey: cfg.GroqAPIKey, Model: cfg.GroqModel, BaseURL: cfg.GroqBaseURL,
		Timeout: cfg.LLMTimeout, RPM: cfg.LLMRPM, MinGap: cfg.LLMMinGap,
	})
	if l.Enabled() {
		log.Printf("llm slow path: Groq %s (budget %d req/min)", cfg.GroqModel, cfg.LLMRPM)
	} else {
		log.Printf("llm slow path: disabled (no GROQ_API_KEY) - deterministic fallback in use")
	}

	host, _ := os.Hostname()
	consumer := stream.NewConsumer(rdb, "writer-"+host, db.WriteBatch)
	go func() {
		if err := consumer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("stream consumer stopped: %v", err)
		}
	}()

	svc := service.New(rdb, db, l, stream.NewProducer(rdb))
	a := api.New(svc, db, func(c context.Context) error { return rdb.Ping(c).Err() }, cfg.CORSOrigin)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	log.Printf("perio-assist backend listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
