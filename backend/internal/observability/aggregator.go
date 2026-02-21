package observability

import (
	"sync"
	"time"
)

// TargetStats holds aggregated metrics for a single target.
type TargetStats struct {
	Name           string  `json:"name"`
	RequestCount   int64   `json:"request_count"`
	ErrorCount     int64   `json:"error_count"`
	TotalLatencyMS float64 `json:"-"`
	AvgLatencyMS   float64 `json:"avg_latency_ms"`
	ErrorRate      float64 `json:"error_rate"`
	RPS            float64 `json:"rps"`
}

// MetricsSnapshot is the periodic summary sent to WebSocket clients.
type MetricsSnapshot struct {
	Timestamp      time.Time      `json:"timestamp"`
	TotalRequests  int64          `json:"total_requests"`
	TotalErrors    int64          `json:"total_errors"`
	AvgLatencyMS   float64        `json:"avg_latency_ms"`
	ErrorRate      float64        `json:"error_rate"`
	ActiveSessions int64          `json:"active_sessions"`
	Targets        []*TargetStats `json:"targets"`
}

// Aggregator maintains in-memory rolling stats for the observability dashboard.
type Aggregator struct {
	mu             sync.Mutex
	targets        map[string]*targetWindow
	windowSize     time.Duration
	activeSessions int64
}

type targetWindow struct {
	entries []windowEntry
}

type windowEntry struct {
	ts         time.Time
	latencyMS  float64
	isError    bool
}

// NewAggregator creates a new Aggregator with a 5 minute rolling window.
func NewAggregator() *Aggregator {
	return &Aggregator{
		targets:    make(map[string]*targetWindow),
		windowSize: 5 * time.Minute,
	}
}

// Record adds an activity entry to the aggregator.
func (a *Aggregator) Record(e ActivityEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	target := e.Target
	if target == "" {
		target = "_gateway"
	}

	w, ok := a.targets[target]
	if !ok {
		w = &targetWindow{}
		a.targets[target] = w
	}

	w.entries = append(w.entries, windowEntry{
		ts:        e.Timestamp,
		latencyMS: e.DurationMS,
		isError:   e.Status == "error",
	})
}

// SetActiveSessions updates the active session count.
func (a *Aggregator) SetActiveSessions(n int64) {
	a.mu.Lock()
	a.activeSessions = n
	a.mu.Unlock()
}

// AdjustActiveSessions atomically adds delta to the session gauge.
func (a *Aggregator) AdjustActiveSessions(delta int64) {
	a.mu.Lock()
	a.activeSessions += delta
	a.mu.Unlock()
}

// Snapshot computes and returns the current metrics snapshot, pruning old entries.
func (a *Aggregator) Snapshot() *MetricsSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-a.windowSize)

	snap := &MetricsSnapshot{
		Timestamp:      now,
		ActiveSessions: a.activeSessions,
	}

	var totalLatency float64

	for name, w := range a.targets {
		// Prune old entries
		pruned := w.entries[:0]
		for _, e := range w.entries {
			if e.ts.After(cutoff) {
				pruned = append(pruned, e)
			}
		}
		w.entries = pruned

		if len(pruned) == 0 {
			continue
		}

		ts := &TargetStats{Name: name}
		for _, e := range pruned {
			ts.RequestCount++
			ts.TotalLatencyMS += e.latencyMS
			if e.isError {
				ts.ErrorCount++
			}
		}
		ts.AvgLatencyMS = ts.TotalLatencyMS / float64(ts.RequestCount)
		if ts.RequestCount > 0 {
			ts.ErrorRate = float64(ts.ErrorCount) / float64(ts.RequestCount)
		}
		ts.RPS = float64(ts.RequestCount) / a.windowSize.Seconds()

		snap.TotalRequests += ts.RequestCount
		snap.TotalErrors += ts.ErrorCount
		totalLatency += ts.TotalLatencyMS

		snap.Targets = append(snap.Targets, ts)
	}

	if snap.TotalRequests > 0 {
		snap.AvgLatencyMS = totalLatency / float64(snap.TotalRequests)
		snap.ErrorRate = float64(snap.TotalErrors) / float64(snap.TotalRequests)
	}

	return snap
}
