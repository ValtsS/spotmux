package metrics

import (
	"log"
	"sync/atomic"
	"time"
)

var (
	SpotsReceived         uint64
	SpotsSent             uint64
	UpstreamErrors        uint64
	DownstreamConnections int64
)

func StartPrinter(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			log.Printf("[METRICS] Spots Recv: %d, Spots Sent: %d, Upstream Errs: %d, Downstream Clients: %d",
				atomic.LoadUint64(&SpotsReceived),
				atomic.LoadUint64(&SpotsSent),
				atomic.LoadUint64(&UpstreamErrors),
				atomic.LoadInt64(&DownstreamConnections))
		}
	}()
}
