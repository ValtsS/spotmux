package spot

import (
	"sync"
	"time"
)

type Deduplicator struct {
	window time.Duration
	// Key: Freq + DXCall + Remarks (simplified)
	lastSeen map[string]time.Time
	mu       sync.Mutex
}

func NewDeduplicator(windowSeconds int) *Deduplicator {
	return &Deduplicator{
		window:   time.Duration(windowSeconds) * time.Second,
		lastSeen: make(map[string]time.Time),
	}
}

func (d *Deduplicator) IsDuplicate(s *Spot) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Create a stable key for comparison
	// Note: Frequency format might vary slightly (e.g. 14025 vs 14025.0)
	// For now, we trust the source cluster's normalization or exact match.
	key := s.Frequency + "|" + s.DXCall + "|" + s.Remarks

	last, exists := d.lastSeen[key]
	if exists && time.Since(last) < d.window {
		return true
	}

	d.lastSeen[key] = time.Now()

	// Periodic cleanup of very old entries to prevent map growth
	// (Simple version: once every 1000 calls or something, but for a 10-client service,
	// even 10,000 keys isn't much memory. Let's keep it simple for now).

	return false
}

// Cleanup removes entries older than the window * 2.
// Can be called periodically from a background goroutine.
func (d *Deduplicator) Cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for k, v := range d.lastSeen {
		if now.Sub(v) > d.window*2 {
			delete(d.lastSeen, k)
		}
	}
}
