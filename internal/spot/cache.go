package spot

import (
	"sync"
)

type Cache struct {
	maxSize int
	spots   []*Spot
	mu      sync.RWMutex
}

func NewCache(size int) *Cache {
	if size <= 0 {
		size = 200
	}
	return &Cache{
		maxSize: size,
		spots:   make([]*Spot, 0, size),
	}
}

func (c *Cache) Add(s *Spot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.spots) >= c.maxSize {
		// Simple way to keep the last N spots.
		// For a small cache (200), shifting is fine.
		c.spots = append(c.spots[1:], s)
	} else {
		c.spots = append(c.spots, s)
	}
}

func (c *Cache) GetAll() []*Spot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to avoid concurrent modification issues
	res := make([]*Spot, len(c.spots))
	copy(res, c.spots)
	return res
}
