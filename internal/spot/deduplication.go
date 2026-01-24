package spot

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

type Deduplicator struct {
	window time.Duration
	// Key: NormalizedFreq + DXCall
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

	// Normalize Frequency (e.g. 14000.0 vs 14000)
	normFreq := s.Frequency
	if f, err := strconv.ParseFloat(s.Frequency, 64); err == nil {
		normFreq = fmt.Sprintf("%.1f", f)
	}

	// Create a stable key for comparison
	// Industry standard: Only Frequency and DXCall are used for deduplication
	key := normFreq + "|" + s.DXCall

	last, exists := d.lastSeen[key]
	if exists && time.Since(last) < d.window {
		return true
	}

	d.lastSeen[key] = time.Now()
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
