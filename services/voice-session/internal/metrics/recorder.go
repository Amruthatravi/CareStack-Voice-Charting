// Package metrics provides lightweight, dependency-free latency tracking
// for the voice pipeline. It keeps a bounded rolling window of samples per
// named stage (e.g. "parse_regex", "speech_to_chart") and can report
// count/min/avg/p50/p95/p99/max on demand via Snapshot().
//
// This is intentionally simple (no Prometheus client, no external time-series
// DB) so it has zero deployment cost — it's meant to answer "is this actually
// fast, right now, in my running app" during development and demos. For a
// production deployment, the natural upgrade path is to keep Record() calls
// exactly as-is and swap Recorder's internals for a Prometheus histogram.
package metrics

import (
	"math"
	"sort"
	"sync"
	"time"
)

// Recorder tracks latency samples per stage using a fixed-size ring buffer,
// so memory use is bounded regardless of how long the service has been running.
type Recorder struct {
	mu      sync.Mutex
	window  int
	samples map[string][]float64 // milliseconds
	cursor  map[string]int
}

// NewRecorder creates a Recorder that keeps the most recent `window` samples
// per stage (older samples are overwritten). window <= 0 defaults to 500.
func NewRecorder(window int) *Recorder {
	if window <= 0 {
		window = 500
	}
	return &Recorder{
		window:  window,
		samples: make(map[string][]float64),
		cursor:  make(map[string]int),
	}
}

// Record adds one latency sample for the given pipeline stage.
// Safe for concurrent use from multiple goroutines/sessions.
func (r *Recorder) Record(stage string, d time.Duration) {
	if d < 0 {
		return // clock skew guard — never record a negative latency
	}
	ms := float64(d.Microseconds()) / 1000.0

	r.mu.Lock()
	defer r.mu.Unlock()

	buf, ok := r.samples[stage]
	if !ok {
		buf = make([]float64, 0, r.window)
	}
	if len(buf) < r.window {
		r.samples[stage] = append(buf, ms)
		return
	}
	i := r.cursor[stage] % r.window
	buf[i] = ms
	r.samples[stage] = buf
	r.cursor[stage]++
}

// StageStats is the reportable summary for one named pipeline stage.
type StageStats struct {
	Stage string  `json:"stage"`
	Count int     `json:"count"`
	MinMs float64 `json:"min_ms"`
	AvgMs float64 `json:"avg_ms"`
	P50Ms float64 `json:"p50_ms"`
	P95Ms float64 `json:"p95_ms"`
	P99Ms float64 `json:"p99_ms"`
	MaxMs float64 `json:"max_ms"`
}

// Snapshot returns current stats for every stage that has at least one
// sample, sorted alphabetically by stage name for stable output.
func (r *Recorder) Snapshot() []StageStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]StageStats, 0, len(r.samples))
	for stage, buf := range r.samples {
		if len(buf) == 0 {
			continue
		}
		cp := make([]float64, len(buf))
		copy(cp, buf)
		sort.Float64s(cp)

		sum := 0.0
		for _, v := range cp {
			sum += v
		}

		out = append(out, StageStats{
			Stage: stage,
			Count: len(cp),
			MinMs: cp[0],
			AvgMs: sum / float64(len(cp)),
			P50Ms: percentile(cp, 0.50),
			P95Ms: percentile(cp, 0.95),
			P99Ms: percentile(cp, 0.99),
			MaxMs: cp[len(cp)-1],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Stage < out[j].Stage })
	return out
}

// percentile expects `sorted` to already be sorted ascending.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
