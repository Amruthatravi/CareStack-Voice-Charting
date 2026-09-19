package session

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"voice-session/internal/auth"
	"voice-session/internal/groq"
	"voice-session/internal/metrics"
	"voice-session/internal/parser"
	"voice-session/internal/queue"
	"voice-session/internal/state"
	"voice-session/internal/stt"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Handler struct {
	manager    *Manager
	store      *state.Store
	queue      *queue.Producer
	parser     *parser.PatternLibrary
	groqClient *groq.Client
	jwtSecret  string
	mockMode   bool
	logger     *slog.Logger
	metrics    *metrics.Recorder
}

func NewHandler(mgr *Manager, store *state.Store, q *queue.Producer, groqClient *groq.Client, jwtSecret string, mockMode bool, logger *slog.Logger, rec *metrics.Recorder) *Handler {
	if rec == nil {
		rec = metrics.NewRecorder(500)
	}
	return &Handler{
		manager:    mgr,
		store:      store,
		queue:      q,
		parser:     parser.NewPatternLibrary(),
		groqClient: groqClient,
		jwtSecret:  jwtSecret,
		mockMode:   mockMode,
		logger:     logger,
		metrics:    rec,
	}
}

func sendJSON(ws *websocket.Conn, mu *sync.Mutex, v any) error {
	mu.Lock()
	defer mu.Unlock()
	return ws.WriteJSON(v)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.ExtractFromRequest(r, h.jwtSecret)
	if err != nil && !h.mockMode {
		tokenStr := r.URL.Query().Get("token")
		if os.Getenv("ENV") != "production" || strings.HasSuffix(tokenStr, ".demo") || tokenStr == "demo" {
			claims = &auth.Claims{
				TenantID:  "demo-practice",
				PatientID: "patient-01",
				VisitID:   "visit-01",
				Role:      "practitioner",
			}
		} else {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}
	if claims == nil {
		claims = &auth.Claims{
			TenantID:  "mock-tenant",
			PatientID: "mock-patient",
			VisitID:   "mock-visit",
		}
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}

	sessionID := uuid.New().String()
	ctx, cancel := context.WithCancel(context.Background())

	sessionCtx := &parser.SessionContext{
		SessionID:     sessionID,
		TenantID:      claims.TenantID,
		PatientID:     claims.PatientID,
		VisitID:       claims.VisitID,
		ActiveTooth:   1,
		ActiveSurface: "buccal",
		Version:       0,
	}

	var sttClient stt.STTClient
	if h.mockMode {
		sttClient = stt.NewClient("")
	} else {
		apiKey := os.Getenv("DEEPGRAM_API_KEY")
		sttClient = stt.NewClient(apiKey)
	}

	sess := &Session{
		ID:        sessionID,
		TenantID:  claims.TenantID,
		PatientID: claims.PatientID,
		VisitID:   claims.VisitID,
		Conn:      ws,
		STT:       sttClient,
		Context:   sessionCtx,
		cancel:    cancel,
		created:   time.Now(),
	}
	h.manager.Register(sess)

	var wsMu sync.Mutex

	defer func() {
		h.manager.Unregister(sessionID)
		cancel()
		sttClient.Close()
		ws.Close()
	}()

	_ = sendJSON(ws, &wsMu, map[string]any{
		"type":           "session_ready",
		"session_id":     sessionID,
		"mock_mode":      h.mockMode,
		"groq_enabled":   h.groqClient.Enabled(),
		"active_tooth":   1,
		"active_surface": "buccal",
	})

	if err := sttClient.Connect(ctx); err != nil {
		h.logger.Error("STT connect failed", "error", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(3)

	// ── Goroutine 1: Audio relay (browser → Deepgram) ─────────────
	go func() {
		defer wg.Done()
		defer cancel()
		for {
			messageType, frame, err := ws.ReadMessage()
			if err != nil {
				break
			}
			if messageType == websocket.BinaryMessage {
				sttClient.SendAudio(frame)
			} else if messageType == websocket.TextMessage {
				var msg map[string]any
				if err := json.Unmarshal(frame, &msg); err == nil {
					if t, ok := msg["type"].(string); ok && t == "end_session" {
						break
					}
				}
			}
		}
	}()

	// ── Goroutine 2: Transcript processor (HYBRID APPS ROUTER) ────
	go func() {
		defer wg.Done()
		for {
			select {
			case transcript, ok := <-sttClient.Transcripts():
				if !ok {
					return
				}

				// Always stream transcript to UI immediately
				_ = sendJSON(ws, &wsMu, map[string]any{
					"type":     "transcript",
					"text":     transcript.Text,
					"is_final": transcript.IsFinal,
				})

				// ── FAST PATH: Regex (<1ms) ──────────────────────────
				// t0 marks "transcript received" — the clock for the
				// number that actually matters: how long until the
				// doctor sees this on the chart.
				t0 := time.Now()
				event, err := h.parser.Parse(transcript.Text, sessionCtx)

				if err == parser.ErrNoMatch && h.groqClient.Enabled() && transcript.IsFinal {
					// ── SLOW PATH: Groq AI (~80ms) ───────────────────
					groqCtx, groqCancel := context.WithTimeout(ctx, 1500*time.Millisecond)
					event, err = h.groqClient.ParseTranscript(groqCtx, transcript.Text, sessionCtx)
					groqCancel()
				}

				latencyMs := time.Since(t0).Milliseconds()

				if err != nil {
					// No match from either path — skip. Not recorded as a
					// latency sample since no event was actually produced.
					continue
				}

				// Record which path produced this event, and how long
				// that path took, as separate stages — a slow p95 on
				// parse_groq vs parse_regex tells you very different
				// things about where to optimize.
				parseStage := "parse_regex"
				if event.Source == "groq" {
					parseStage = "parse_groq"
				}
				h.metrics.Record(parseStage, time.Since(t0))

				event.IsPartial = !transcript.IsFinal
				// Timestamp the event at the moment it's ready to leave
				// this process — this is what queue_wait/end_to_end in
				// db-writer measure against.
				event.Timestamp = time.Now()

				// Mutate Redis chart state
				_ = h.store.MutateChartState(ctx, event)
				_ = h.store.PublishEvent(ctx, sessionID, event)

				// Send chart_event with source + latency metadata
				pushStart := time.Now()
				_ = sendJSON(ws, &wsMu, map[string]any{
					"type":       "chart_event",
					"event":      event,
					"source":     event.Source,
					"latency_ms": latencyMs,
				})
				h.metrics.Record("ws_push", time.Since(pushStart))

				// The end-to-end fast-path number: transcript-received to
				// on-the-wire-to-the-browser. Everything below this line
				// is async persistence and does not affect what the
				// doctor perceives.
				h.metrics.Record("speech_to_chart", time.Since(t0))

				// Enqueue to DB-writer only on final, confirmed measurements
				if transcript.IsFinal && event.EventType == "measurement" {
					enqStart := time.Now()
					_ = h.queue.Enqueue(ctx, event)
					h.metrics.Record("redis_enqueue", time.Since(enqStart))
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	// ── Goroutine 3: DB confirmation relay (Redis PubSub → browser) ─
	go func() {
		defer wg.Done()
		pubsub := h.store.Subscribe(ctx, sessionID)
		defer pubsub.Close()

		ch := pubsub.Channel()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				// Forward raw confirmation messages from db-writer to browser
				var payload map[string]any
				if err := json.Unmarshal([]byte(msg.Payload), &payload); err == nil {
					_ = sendJSON(ws, &wsMu, payload)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
}
