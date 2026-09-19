package state

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"voice-session/internal/parser"
)

type Store struct {
	client *redis.Client
	logger *slog.Logger
}

func NewStore(redisURL string) (*Store, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &Store{
		client: client,
		logger: slog.Default(),
	}, nil
}

// Client exposes the underlying Redis client for use by other packages (e.g. queue producer).
func (s *Store) Client() *redis.Client {
	return s.client
}


func chartKey(tenantID, patientID, visitID string) string {
	return fmt.Sprintf("chart:%s:%s:%s", tenantID, patientID, visitID)
}

func versionKey(tenantID, patientID, visitID string) string {
	return fmt.Sprintf("chart:%s:%s:%s:version", tenantID, patientID, visitID)
}

func sessionKey(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

func pubsubChannel(sessionID string) string {
	return fmt.Sprintf("ui:%s", sessionID)
}

func (s *Store) MutateChartState(ctx context.Context, event *parser.ChartEvent) error {
	key := chartKey(event.TenantID, event.PatientID, event.VisitID)
	vkey := versionKey(event.TenantID, event.PatientID, event.VisitID)

	for k, v := range event.Measurements {
		field := fmt.Sprintf("t%d_%s_%s", event.ToothNum, event.Surface, k)
		valBytes, _ := json.Marshal(v)
		
		script := `
local key = KEYS[1]
local vkey = KEYS[2]
local field = ARGV[1]
local value = ARGV[2]
redis.call('HSET', key, field, value)
redis.call('EXPIRE', key, 86400)
local v = redis.call('INCR', vkey)
redis.call('EXPIRE', vkey, 86400)
return v
`
		_, err := s.client.Eval(ctx, script, []string{key, vkey}, field, string(valBytes)).Result()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetChartState(ctx context.Context, tenantID, patientID, visitID string) (map[string]string, error) {
	return s.client.HGetAll(ctx, chartKey(tenantID, patientID, visitID)).Result()
}

func (s *Store) PublishEvent(ctx context.Context, sessionID string, event *parser.ChartEvent) error {
	data, _ := json.Marshal(event)
	return s.client.Publish(ctx, pubsubChannel(sessionID), data).Err()
}

func (s *Store) Subscribe(ctx context.Context, sessionID string) *redis.PubSub {
	return s.client.Subscribe(ctx, pubsubChannel(sessionID))
}

func (s *Store) SetSessionMeta(ctx context.Context, sessionID string, meta map[string]string) error {
	key := sessionKey(sessionID)
	err := s.client.HSet(ctx, key, meta).Err()
	if err == nil {
		s.client.Expire(ctx, key, 8*3600*1000*1000*1000) // 8 hours
	}
	return err
}

func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	return s.client.Del(ctx, sessionKey(sessionID)).Err()
}
