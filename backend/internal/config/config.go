package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	RedisAddr   string
	CORSOrigin  string

	GroqAPIKey  string
	GroqModel   string
	GroqBaseURL string
	LLMRPM      int           // global requests/minute budget for the slow path
	LLMMinGap   time.Duration // min gap between LLM calls for one session
	LLMTimeout  time.Duration
}

func Load() Config {
	return Config{
		Port:        env("PORT", "8080"),
		DatabaseURL: env("DATABASE_URL", "postgres://perio:perio@localhost:5432/perio?sslmode=disable"),
		RedisAddr:   env("REDIS_ADDR", "localhost:6379"),
		CORSOrigin:  env("CORS_ORIGIN", "*"),
		GroqAPIKey:  env("GROQ_API_KEY", ""),
		GroqModel:   env("GROQ_MODEL", "llama-3.1-8b-instant"),
		GroqBaseURL: env("GROQ_BASE_URL", "https://api.groq.com/openai/v1"),
		LLMRPM:      envInt("LLM_RPM", 20),
		LLMMinGap:   time.Duration(envInt("LLM_MIN_GAP_MS", 1500)) * time.Millisecond,
		LLMTimeout:  time.Duration(envInt("LLM_TIMEOUT_MS", 2500)) * time.Millisecond,
	}
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}
