package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"perioassist/internal/service"
	"perioassist/internal/store"
)

type API struct {
	svc  *service.Service
	db   *store.Store
	ping func(context.Context) error
	cors string
	hub  *hub
}

func New(svc *service.Service, db *store.Store, redisPing func(context.Context) error, corsOrigin string) *API {
	return &API{svc: svc, db: db, ping: redisPing, cors: corsOrigin, hub: &hub{rooms: map[string]map[*client]struct{}{}}}
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("POST /api/sessions", a.createSession)
	mux.HandleFunc("GET /api/sessions", a.listSessions)
	mux.HandleFunc("GET /api/sessions/{id}", a.getSession)
	mux.HandleFunc("POST /api/sessions/{id}/utterances", a.utterance)
	mux.HandleFunc("GET /api/sessions/{id}/audit", a.audit)
	mux.HandleFunc("GET /api/sessions/{id}/export", a.export)
	mux.HandleFunc("GET /ws", a.ws)
	return a.withCORS(mux)
}

func (a *API) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", a.cors)
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (a *API) sessionID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeErr(w, http.StatusBadRequest, "invalid session id")
		return "", false
	}
	ok, err := a.db.SessionExists(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return "", false
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "session not found")
		return "", false
	}
	return id, true
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	st := map[string]any{"postgres": a.db.Ping(ctx) == nil, "redis": a.ping(ctx) == nil, "llm": a.svc.LLMEnabled()}
	code := http.StatusOK
	if !st["postgres"].(bool) || !st["redis"].(bool) {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, st)
}

func (a *API) createSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PatientLabel string `json:"patientLabel"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body)
	if len(body.PatientLabel) > 120 {
		body.PatientLabel = body.PatientLabel[:120]
	}
	s, err := a.db.CreateSession(r.Context(), body.PatientLabel)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (a *API) listSessions(w http.ResponseWriter, r *http.Request) {
	list, err := a.db.ListSessions(r.Context(), 20)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *API) getSession(w http.ResponseWriter, r *http.Request) {
	id, ok := a.sessionID(w, r)
	if !ok {
		return
	}
	resp, err := a.svc.State(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) utterance(w http.ResponseWriter, r *http.Request) {
	id, ok := a.sessionID(w, r)
	if !ok {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<12)).Decode(&body); err != nil || len(body.Text) > 400 {
		writeErr(w, http.StatusBadRequest, "body must be {\"text\": \"...\"} (max 400 chars)")
		return
	}
	resp, err := a.svc.Handle(r.Context(), id, body.Text)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.hub.broadcast(id, resp)
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) audit(w http.ResponseWriter, r *http.Request) {
	id, ok := a.sessionID(w, r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := a.db.ListAudit(r.Context(), id, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (a *API) export(w http.ResponseWriter, r *http.Request) {
	id, ok := a.sessionID(w, r)
	if !ok {
		return
	}
	resp, err := a.svc.State(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="perio-chart-`+id[:8]+`.json"`)
	writeJSON(w, http.StatusOK, map[string]any{"sessionId": id, "exportedAt": time.Now().UTC(), "chart": resp.Chart})
}

// ---- WebSocket ----

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 8192,
	CheckOrigin:     func(r *http.Request) bool { return true }, // dev; nginx is same-origin in Docker
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

type hub struct {
	mu    sync.Mutex
	rooms map[string]map[*client]struct{}
}

func (h *hub) join(id string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[id] == nil {
		h.rooms[id] = map[*client]struct{}{}
	}
	h.rooms[id][c] = struct{}{}
}

func (h *hub) leave(id string, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room := h.rooms[id]; room != nil {
		delete(room, c)
		if len(room) == 0 {
			delete(h.rooms, id)
		}
	}
}

type envelope struct {
	Type    string            `json:"type"`
	Data    *service.Response `json:"data,omitempty"`
	Message string            `json:"message,omitempty"`
}

func (h *hub) broadcast(id string, resp *service.Response) {
	b, _ := json.Marshal(envelope{Type: "state", Data: resp})
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.rooms[id] {
		select {
		case c.send <- b:
		default: // slow client: drop it
			close(c.send)
			delete(h.rooms[id], c)
		}
	}
}

func (a *API) ws(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("session")
	if !uuidRe.MatchString(id) {
		http.Error(w, "invalid session", http.StatusBadRequest)
		return
	}
	if ok, err := a.db.SessionExists(r.Context(), id); err != nil || !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 32)}
	a.hub.join(id, c)
	defer func() { a.hub.leave(id, c); conn.Close() }()

	go func() { // single writer
		t := time.NewTicker(25 * time.Second)
		defer t.Stop()
		for {
			select {
			case msg, ok := <-c.send:
				if !ok {
					_ = conn.WriteMessage(websocket.CloseMessage, nil)
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if conn.WriteMessage(websocket.TextMessage, msg) != nil {
					return
				}
			case <-t.C:
				_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if conn.WriteMessage(websocket.PingMessage, nil) != nil {
					return
				}
			}
		}
	}()

	if st, err := a.svc.State(r.Context(), id); err == nil {
		b, _ := json.Marshal(envelope{Type: "state", Data: st})
		c.send <- b
	}

	conn.SetReadLimit(4096)
	_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(70 * time.Second)) })

	for {
		var in struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := conn.ReadJSON(&in); err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
		if in.Type != "utterance" || len(in.Text) > 400 {
			continue
		}
		resp, err := a.svc.Handle(context.Background(), id, in.Text)
		if err != nil {
			log.Printf("ws handle: %v", err)
			b, _ := json.Marshal(envelope{Type: "error", Message: "could not process utterance"})
			select {
			case c.send <- b:
			default:
			}
			continue
		}
		a.hub.broadcast(id, resp)
	}
}
