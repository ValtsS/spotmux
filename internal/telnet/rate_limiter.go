package telnet

import (
	"sync"
	"time"
)

type RateLimiter struct {
	rate       int
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

func NewRateLimiter(ratePerMinute int) *RateLimiter {
	return &RateLimiter{
		rate:       ratePerMinute,
		tokens:     float64(ratePerMinute),
		lastUpdate: time.Now(),
	}
}

func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastUpdate).Minutes()
	r.tokens += elapsed * float64(r.rate)
	if r.tokens > float64(r.rate) {
		r.tokens = float64(r.rate)
	}
	r.lastUpdate = now

	if r.tokens >= 1 {
		r.tokens--
		return true
	}

	return false
}
