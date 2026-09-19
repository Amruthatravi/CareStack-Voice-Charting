package stt

import "context"

type Transcript struct {
	Text    string
	IsFinal bool
}

type STTClient interface {
	Connect(ctx context.Context) error
	SendAudio(frame []byte) error
	Transcripts() <-chan Transcript
	Close() error
}

func NewClient(apiKey string) STTClient {
	if apiKey == "" {
		return NewMockClient()
	}
	return NewDeepgramClient(apiKey)
}
