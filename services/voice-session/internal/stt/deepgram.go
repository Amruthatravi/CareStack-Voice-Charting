package stt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type deepgramResponse struct {
	Type    string `json:"type"`
	Channel struct {
		Alternatives []struct {
			Transcript string  `json:"transcript"`
			Confidence float64 `json:"confidence"`
		} `json:"alternatives"`
	} `json:"channel"`
	IsFinal bool `json:"is_final"`
}

type deepgramClient struct {
	apiKey       string
	conn         *websocket.Conn
	transcriptCh chan Transcript
	mu           sync.Mutex
	closed       bool
}

func NewDeepgramClient(apiKey string) STTClient {
	return &deepgramClient{
		apiKey:       apiKey,
		transcriptCh: make(chan Transcript, 100),
	}
}

func (c *deepgramClient) Connect(ctx context.Context) error {
	u, err := url.Parse("wss://api.deepgram.com/v1/listen")
	if err != nil {
		return err
	}

	q := u.Query()
	q.Set("model", "nova-2-medical")
	q.Set("encoding", "linear16")
	q.Set("sample_rate", "16000")
	q.Set("channels", "1")
	q.Set("interim_results", "true")
	q.Set("endpointing", "150")
	q.Set("utterance_end_ms", "1000")
	q.Set("smart_format", "true")
	q.Set("punctuate", "false")
	// Medical + dental keyword boosting
	q.Set("keywords",
		"periodontal:5,furcation:8,suppuration:8,recession:7,"+
			"bleeding:4,buccal:5,lingual:5,mesial:5,distal:5,mobility:6,"+
			"plaque:4,calculus:5,probing:4,pocket:4,attachment:4,"+
			"class:3,grade:3,furcation:8,bop:6,suppurate:7")
	u.RawQuery = q.Encode()

	headers := http.Header{}
	headers.Add("Authorization", "Token "+c.apiKey)

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			return fmt.Errorf("Deepgram WebSocket handshake failed (%s): %s", resp.Status, strings.TrimSpace(string(body)))
		}
		return fmt.Errorf("Deepgram WebSocket handshake failed: %w", err)
	}
	c.conn = conn

	go c.readLoop()

	return nil
}

func (c *deepgramClient) readLoop() {
	defer close(c.transcriptCh)

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("Deepgram read error: %v", err)
			return
		}

		var resp deepgramResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			continue
		}

		if len(resp.Channel.Alternatives) > 0 {
			alt := resp.Channel.Alternatives[0]
			// Filter out low-confidence transcripts
			if alt.Transcript != "" && alt.Confidence >= 0.60 {
				c.transcriptCh <- Transcript{
					Text:    alt.Transcript,
					IsFinal: resp.IsFinal,
				}
			}
		}
	}
}

func (c *deepgramClient) SendAudio(frame []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.conn == nil {
		return nil
	}
	return c.conn.WriteMessage(websocket.BinaryMessage, frame)
}

func (c *deepgramClient) Transcripts() <-chan Transcript {
	return c.transcriptCh
}

func (c *deepgramClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	closeMsg := []byte(`{"type":"CloseStream"}`)
	if c.conn != nil {
		c.conn.WriteMessage(websocket.TextMessage, closeMsg)
		return c.conn.Close()
	}
	return nil
}
