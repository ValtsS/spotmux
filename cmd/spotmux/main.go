package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/valtss/spotmux/internal/cluster"
	"github.com/valtss/spotmux/internal/config"
	"github.com/valtss/spotmux/internal/metrics"
	"github.com/valtss/spotmux/internal/spot"
	"github.com/valtss/spotmux/internal/telnet"
)

func main() {
	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting DX Aggregator for callsign %s", cfg.ServiceCallsign)

	cache := spot.NewCache(cfg.SpotCacheSize)
	dedup := spot.NewDeduplicator(cfg.DuplicateWindowSeconds)

	// Start metrics
	metrics.StartPrinter(30 * time.Second)

	// Start Upstream Manager
	upstream := cluster.NewUpstreamManager(cfg.ServiceCallsign, cfg.UpstreamClusters)
	upstreamSpots := upstream.Start()

	// Start Telnet Server
	server := telnet.NewServer(cfg.ListenPort, cfg.MaxDownstreamConnections, cfg.ServiceCallsign, cache, dedup, cfg.MaxSpotsPerMinute)
	outboundSpots, err := server.Start()
	if err != nil {
		log.Fatalf("Failed to start telnet server: %v", err)
	}

	// Glue code
	go func() {
		for s := range upstreamSpots {
			atomic.AddUint64(&metrics.SpotsReceived, 1)
			server.HandleInboundSpot(s)
		}
	}()

	go func() {
		for s := range outboundSpots {
			atomic.AddUint64(&metrics.SpotsSent, 1)
			server.HandleInboundSpot(s) // Echo locally
			upstream.BroadcastSpot(s)   // Send upstream
		}
	}()

	// Periodic cleanup for deduplicator
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.DuplicateWindowSeconds) * 2 * time.Second)
		for range ticker.C {
			dedup.Cleanup()
		}
	}()

	// Signal handling (KISS)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Service is running. Press Ctrl+C to stop.")
	<-sigChan
	log.Println("Shutting down...")
	// Sockets will close automatically on exit as per KISS requirement
}
