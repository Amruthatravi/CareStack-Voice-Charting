package groq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"voice-session/internal/parser"
)

const (
	groqAPIURL  = "https://api.groq.com/openai/v1/chat/completions"
	groqTimeout = 1200 * time.Millisecond
)

func getGroqModel() string {
	if m := os.Getenv("GROQ_MODEL"); m != "" {
		return m
	}
	return "qwen/qwen3.8-27b"
}

var ErrGroqTimeout = errors.New("groq: inference timeout")
var ErrGroqDisabled = errors.New("groq: no API key configured")

type Client struct {
	apiKey string
	hc     *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		hc: &http.Client{
			Timeout: groqTimeout,
		},
	}
}

func (c *Client) Enabled() bool {
	return c.apiKey != ""
}

type groqRequest struct {
	Model    string        `json:"model"`
	Messages []groqMessage `json:"messages"`
	MaxTokens int         `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type groqParsed struct {
	EventType    string         `json:"event_type"`
	ToothNum     *int           `json:"tooth_num"`
	Surface      *string        `json:"surface"`
	Measurements map[string]any `json:"measurements"`
	Confidence   float64        `json:"confidence"`
}

func (c *Client) ParseTranscript(ctx context.Context, transcript string, sessionCtx *parser.SessionContext) (*parser.ChartEvent, error) {
	if !c.Enabled() {
		return nil, ErrGroqDisabled
	}

	userMsg := fmt.Sprintf(
		"Active tooth: %d, surface: %s\nTranscript: %s",
		sessionCtx.ActiveTooth, sessionCtx.ActiveSurface, transcript,
	)

	reqBody := groqRequest{
		Model: getGroqModel(),
		Messages: []groqMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMsg},
		},
		MaxTokens:   256,
		Temperature: 0.0,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrGroqTimeout
		}
		return nil, fmt.Errorf("groq: http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("groq: status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var groqResp groqResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return nil, fmt.Errorf("groq: decode response: %w", err)
	}

	if len(groqResp.Choices) == 0 {
		return nil, errors.New("groq: empty response")
	}

	var parsed groqParsed
	if err := json.Unmarshal([]byte(groqResp.Choices[0].Message.Content), &parsed); err != nil {
		return nil, fmt.Errorf("groq: parse JSON output: %w", err)
	}

	if parsed.EventType == "none" || parsed.EventType == "" {
		return nil, parser.ErrNoMatch
	}

	// Build ChartEvent from Groq output
	sessionCtx.Version++
	event := &parser.ChartEvent{
		SessionID:    sessionCtx.SessionID,
		TenantID:     sessionCtx.TenantID,
		PatientID:    sessionCtx.PatientID,
		EventType:    parsed.EventType,
		ToothNum:     sessionCtx.ActiveTooth,
		Surface:      sessionCtx.ActiveSurface,
		Measurements: make(map[string]any),
		Confidence:   parsed.Confidence,
		SourceText:   transcript,
		Version:      sessionCtx.Version,
		Source:       "groq",
	}

	if parsed.ToothNum != nil && *parsed.ToothNum > 0 && *parsed.ToothNum <= 32 {
		event.ToothNum = *parsed.ToothNum
		sessionCtx.ActiveTooth = *parsed.ToothNum
	}
	if parsed.Surface != nil && (*parsed.Surface == "buccal" || *parsed.Surface == "lingual") {
		event.Surface = *parsed.Surface
		sessionCtx.ActiveSurface = *parsed.Surface
	}

	// Copy measurements, normalizing pocket_depth from []interface{} to []int
	for k, v := range parsed.Measurements {
		if v == nil {
			continue
		}
		if k == "pocket_depth" {
			if arr, ok := v.([]interface{}); ok && len(arr) == 3 {
				pd := make([]int, 3)
				for i, val := range arr {
					switch n := val.(type) {
					case float64:
						pd[i] = int(n)
					}
				}
				event.Measurements[k] = pd
			}
		} else {
			event.Measurements[k] = v
		}
	}

	return event, nil
}
