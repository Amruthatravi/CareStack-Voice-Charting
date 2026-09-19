package stt

import (
	"context"
	"time"
)

// mockScript simulates a realistic full periodontal charting session
// covering all measurement types, corrections, and natural speech variations.
var mockScript = []struct {
	delay      time.Duration
	transcript string
	isFinal    bool
}{
	// === Tooth 1 ===
	{900 * time.Millisecond, "tooth one", true},
	{600 * time.Millisecond, "two", false},
	{600 * time.Millisecond, "two three two", true},
	{700 * time.Millisecond, "bleeding", true},
	{700 * time.Millisecond, "next", true},
	// === Tooth 2 ===
	{800 * time.Millisecond, "tooth two", true},
	{700 * time.Millisecond, "three two three", true},
	{700 * time.Millisecond, "next", true},
	// === Tooth 3 ===
	{800 * time.Millisecond, "tooth three", true},
	{700 * time.Millisecond, "four three four", true},
	{700 * time.Millisecond, "bleeding", true},
	{700 * time.Millisecond, "recession two", true},
	{700 * time.Millisecond, "next", true},
	// === Tooth 4 — word numbers ===
	{800 * time.Millisecond, "tooth four", true},
	{700 * time.Millisecond, "three three two", true},
	{700 * time.Millisecond, "next", true},
	// === Tooth 5 — furcation ===
	{800 * time.Millisecond, "tooth five", true},
	{700 * time.Millisecond, "five four six", true},
	{700 * time.Millisecond, "bleeding", true},
	{700 * time.Millisecond, "furcation class two", true},
	{700 * time.Millisecond, "next", true},
	// === Tooth 6 — mobility + suppuration ===
	{800 * time.Millisecond, "tooth six", true},
	{700 * time.Millisecond, "six five seven", true},
	{700 * time.Millisecond, "mobility one", true},
	{700 * time.Millisecond, "suppuration", true},
	{700 * time.Millisecond, "next", true},
	// === Tooth 7 — plaque ===
	{800 * time.Millisecond, "tooth seven", true},
	{700 * time.Millisecond, "three two three", true},
	{700 * time.Millisecond, "plaque positive", true},
	{700 * time.Millisecond, "next", true},
	// === Tooth 8 — correction scratch that ===
	{800 * time.Millisecond, "tooth eight", true},
	{700 * time.Millisecond, "four three four", true},
	{700 * time.Millisecond, "scratch that", true},
	{700 * time.Millisecond, "three three three", true},
	{700 * time.Millisecond, "next", true},
	// === Tooth 14 — site correction ===
	{800 * time.Millisecond, "tooth fourteen", true},
	{700 * time.Millisecond, "three two four", true},
	{700 * time.Millisecond, "bleeding", true},
	{700 * time.Millisecond, "make site two a five", true},
	{700 * time.Millisecond, "mobility one", true},
	{700 * time.Millisecond, "next", true},
	// === Tooth 19 — lingual surface ===
	{800 * time.Millisecond, "tooth nineteen", true},
	{700 * time.Millisecond, "four four five", true},
	{700 * time.Millisecond, "bleeding", true},
	{700 * time.Millisecond, "furcation class three", true},
	{700 * time.Millisecond, "lingual", true},
	{700 * time.Millisecond, "three three four", true},
	{700 * time.Millisecond, "next", true},
	// === Tooth 30 — deep pockets ===
	{800 * time.Millisecond, "tooth thirty", true},
	{700 * time.Millisecond, "seven six eight", true},
	{700 * time.Millisecond, "bop", true},
	{700 * time.Millisecond, "recession three", true},
	{700 * time.Millisecond, "furcation class two", true},
	{700 * time.Millisecond, "next", true},
	// Pause then restart from tooth 1
	{2000 * time.Millisecond, "", false},
}

type mockClient struct {
	transcriptCh chan Transcript
	cancel       context.CancelFunc
}

func NewMockClient() STTClient {
	return &mockClient{
		transcriptCh: make(chan Transcript, 100),
	}
}

func (m *mockClient) Connect(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	go func() {
		defer close(m.transcriptCh)
		for {
			for _, step := range mockScript {
				select {
				case <-ctx.Done():
					return
				case <-time.After(step.delay):
					if step.transcript != "" {
						m.transcriptCh <- Transcript{
							Text:    step.transcript,
							IsFinal: step.isFinal,
						}
					}
				}
			}
		}
	}()
	return nil
}

func (m *mockClient) SendAudio(frame []byte) error {
	return nil
}

func (m *mockClient) Transcripts() <-chan Transcript {
	return m.transcriptCh
}

func (m *mockClient) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	return nil
}
