package parser_test

import (
	"reflect"
	"testing"
	"time"

	"voice-session/internal/parser"
)

func TestParser(t *testing.T) {
	pl := parser.NewPatternLibrary()

	tests := []struct {
		name       string
		transcript string
		wantEvent  *parser.ChartEvent
		wantErr    error
	}{
		{
			name:       "three two four",
			transcript: "three two four",
			wantEvent: &parser.ChartEvent{
				EventType: "measurement",
				Measurements: map[string]any{
					"pocket_depth": []int{3, 2, 4},
				},
			},
			wantErr: nil,
		},
		{
			name:       "three two four bleeding",
			transcript: "three two four bleeding",
			wantEvent: &parser.ChartEvent{
				EventType: "measurement",
				Measurements: map[string]any{
					"pocket_depth": []int{3, 2, 4},
					"bleeding":     true,
				},
			},
			wantErr: nil,
		},
		{
			name:       "tooth fourteen",
			transcript: "tooth fourteen",
			wantEvent: &parser.ChartEvent{
				EventType: "navigate",
				ToothNum:  14,
				Measurements: map[string]any{},
			},
			wantErr: nil,
		},
		{
			name:       "tooth thirty two",
			transcript: "tooth thirty two",
			wantEvent: &parser.ChartEvent{
				EventType: "navigate",
				ToothNum:  32,
				Measurements: map[string]any{},
			},
			wantErr: nil,
		},
		{
			name:       "tooth thirty-two hyphenated",
			transcript: "tooth thirty-two",
			wantEvent: &parser.ChartEvent{
				EventType: "navigate",
				ToothNum:  32,
				Measurements: map[string]any{},
			},
			wantErr: nil,
		},
		{
			name:       "tooth twenty one",
			transcript: "tooth twenty one",
			wantEvent: &parser.ChartEvent{
				EventType: "navigate",
				ToothNum:  21,
				Measurements: map[string]any{},
			},
			wantErr: nil,
		},
		{
			name:       "recession two",
			transcript: "recession two",
			wantEvent: &parser.ChartEvent{
				Measurements: map[string]any{
					"recession": 2,
				},
			},
			wantErr: nil,
		},
		{
			name:       "furcation class two",
			transcript: "furcation class two",
			wantEvent: &parser.ChartEvent{
				Measurements: map[string]any{
					"furcation": 2,
				},
			},
			wantErr: nil,
		},
		{
			name:       "mobility one",
			transcript: "mobility one",
			wantEvent: &parser.ChartEvent{
				Measurements: map[string]any{
					"mobility": 1,
				},
			},
			wantErr: nil,
		},
		{
			name:       "scratch that",
			transcript: "scratch that",
			wantEvent: &parser.ChartEvent{
				EventType: "correction",
				Confidence: 0.97,
				Measurements: map[string]any{
					"undo_last": true,
				},
			},
			wantErr: nil,
		},
		{
			name:       "make site two a five",
			transcript: "make site two a five",
			wantEvent: &parser.ChartEvent{
				EventType: "correction",
				Confidence: 0.97,
				Measurements: map[string]any{
					"site_correction": map[string]int{
						"site_index": 1,
						"new_value":  5,
					},
				},
			},
			wantErr: nil,
		},
		{
			name:       "next",
			transcript: "next",
			wantEvent: &parser.ChartEvent{
				EventType: "navigate",
				Measurements: map[string]any{
					"auto_advance": true,
				},
			},
			wantErr: nil,
		},
		{
			name:       "buccal",
			transcript: "buccal",
			wantEvent: &parser.ChartEvent{
				Surface: "buccal",
				Measurements: map[string]any{},
			},
			wantErr: nil,
		},
		{
			name:       "three-two-four",
			transcript: "three-two-four",
			wantEvent: &parser.ChartEvent{
				EventType: "measurement",
				Measurements: map[string]any{
					"pocket_depth": []int{3, 2, 4},
				},
			},
			wantErr: nil,
		},
		{
			name:       "suppuration",
			transcript: "suppuration",
			wantEvent: &parser.ChartEvent{
				Measurements: map[string]any{
					"suppuration": true,
				},
			},
			wantErr: nil,
		},
		{
			name:       "bleeding on probing",
			transcript: "bleeding on probing",
			wantEvent: &parser.ChartEvent{
				Measurements: map[string]any{
					"bleeding": true,
				},
			},
			wantErr: nil,
		},
		{
			name:       "furcation grade iii",
			transcript: "furcation grade iii",
			wantEvent: &parser.ChartEvent{
				Measurements: map[string]any{
					"furcation": 3,
				},
			},
			wantErr: nil,
		},
		{
			name:       "Empty string",
			transcript: "",
			wantEvent:  nil,
			wantErr:    parser.ErrNoMatch,
		},
		{
			// Regression test: a tooth reference and a measurement spoken in
			// the same breath ("tooth one, bleeding") must still classify as
			// a measurement event, not "navigate" — otherwise the frontend's
			// navigate branch discards the measurement data entirely.
			name:       "tooth one bleeding combined",
			transcript: "tooth one, bleeding",
			wantEvent: &parser.ChartEvent{
				EventType: "measurement",
				ToothNum:  1,
				Measurements: map[string]any{
					"bleeding": true,
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &parser.SessionContext{
				ActiveTooth:   1,
				ActiveSurface: "buccal",
				Version:       0,
			}

			// Pre-set some expectations that the parser naturally sets
			if tt.wantEvent != nil {
				tt.wantEvent.SessionID = ctx.SessionID
				tt.wantEvent.TenantID = ctx.TenantID
				tt.wantEvent.PatientID = ctx.PatientID
				tt.wantEvent.SourceText = tt.transcript
				if tt.wantEvent.ToothNum == 0 && tt.name != "tooth fourteen" {
					tt.wantEvent.ToothNum = 1
				}
				if tt.wantEvent.Surface == "" && tt.name != "buccal" {
					tt.wantEvent.Surface = "buccal"
				}
				if tt.wantEvent.Confidence == 0 {
					tt.wantEvent.Confidence = 1.0
				}
				tt.wantEvent.Version = 1
			}

			got, err := pl.Parse(tt.transcript, ctx)
			if err != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if got != nil && tt.wantEvent != nil {
				got.Timestamp = time.Time{}
				tt.wantEvent.Timestamp = time.Time{}
				
				if !reflect.DeepEqual(got, tt.wantEvent) {
					t.Errorf("Parse() got = %+v, want %+v", got, tt.wantEvent)
				}
			}
		})
	}
}
