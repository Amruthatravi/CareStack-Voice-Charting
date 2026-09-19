package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"voice-session/internal/parser"
	"voice-session/internal/queue"
	"voice-session/internal/state"
	"voice-session/internal/stt"
)

type Session struct {
	ID        string
	TenantID  string
	PatientID string
	VisitID   string
	Conn      *websocket.Conn
	STT       stt.STTClient
	Context   *parser.SessionContext
	cancel    context.CancelFunc
	created   time.Time
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	store    *state.Store
	queue    *queue.Producer
	logger   *slog.Logger
}

func NewManager(store *state.Store, queue *queue.Producer, logger *slog.Logger) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		store:    store,
		queue:    queue,
		logger:   logger,
	}
}

func (m *Manager) Register(s *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
}

func (m *Manager) Unregister(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[sessionID]; ok {
		if s.cancel != nil {
			s.cancel()
		}
		if s.STT != nil {
			s.STT.Close()
		}
		if s.Conn != nil {
			s.Conn.Close()
		}
		delete(m.sessions, sessionID)
	}
}

func (m *Manager) Get(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	return s, ok
}

func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}
