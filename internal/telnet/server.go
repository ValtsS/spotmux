package telnet

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/valtss/spotmux/internal/spot"
)

type Client struct {
	conn     net.Conn
	callsign string
	spotsOn  bool
}

type Server struct {
	port          int
	maxConns      int
	serviceCall   string
	clients       map[net.Conn]*Client
	mu            sync.Mutex
	spotChan      chan *spot.Spot
	outboundSpots chan *spot.Spot
	cache         *spot.Cache
	deduplicator  *spot.Deduplicator
	rateLimiter   *RateLimiter
}

func NewServer(port int, maxConns int, serviceCall string, cache *spot.Cache, dedup *spot.Deduplicator, rateLimit int) *Server {
	return &Server{
		port:          port,
		maxConns:      maxConns,
		serviceCall:   strings.ToUpper(serviceCall),
		clients:       make(map[net.Conn]*Client),
		spotChan:      make(chan *spot.Spot, 100),
		outboundSpots: make(chan *spot.Spot, 10),
		cache:         cache,
		deduplicator:  dedup,
		rateLimiter:   NewRateLimiter(rateLimit),
	}
}

func (s *Server) Start() (<-chan *spot.Spot, error) {
	addr := fmt.Sprintf(":%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	log.Printf("Telnet server listening on %s", addr)

	go s.acceptLoop(ln)
	go s.broadcastLoop()

	return s.outboundSpots, nil
}

func (s *Server) HandleInboundSpot(sp *spot.Spot) {
	s.spotChan <- sp
}

func (s *Server) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}

		s.mu.Lock()
		if len(s.clients) >= s.maxConns {
			s.mu.Unlock()
			fmt.Fprintln(conn, "Server full. Try again later.")
			conn.Close()
			continue
		}
		client := &Client{conn: conn, spotsOn: true}
		s.clients[conn] = client
		s.mu.Unlock()

		go s.handleClient(client)
	}
}

func (s *Server) handleClient(c *Client) {
	defer func() {
		s.mu.Lock()
		delete(s.clients, c.conn)
		s.mu.Unlock()
		c.conn.Close()
	}()

	fmt.Fprintf(c.conn, "Welcome to SpotMux DX Aggregator\nDXSpider V1.55 build 0.000 (standard) %s\n", s.serviceCall)
	fmt.Fprint(c.conn, "login: ")

	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if c.callsign == "" {
			c.callsign = strings.ToUpper(line)
			fmt.Fprintf(c.conn, "Hello %s. Type HELP for commands.\n", c.callsign)

			// Send cache
			for _, sp := range s.cache.GetAll() {
				fmt.Fprintln(c.conn, sp.String())
			}
			fmt.Fprintf(c.conn, "de %s > ", s.serviceCall)
			continue
		}

		s.processCommand(c, line)
		fmt.Fprintf(c.conn, "de %s > ", s.serviceCall)
	}
}

func (s *Server) processCommand(c *Client, line string) {
	lower := strings.ToLower(line)
	parts := strings.Fields(line)
	cmd := strings.ToLower(parts[0])

	switch {
	case cmd == "dx":
		// dx frequency callsign remarks
		if len(parts) < 3 {
			fmt.Fprintln(c.conn, "Usage: DX frequency callsign [remarks]")
			return
		}
		sp := &spot.Spot{
			Spotter:    c.callsign,
			Frequency:  parts[1],
			DXCall:     parts[2],
			Remarks:    strings.Join(parts[3:], " "),
			Time:       time.Now().Format("1504Z"),
			ReceivedAt: time.Now(),
		}
		if !s.rateLimiter.Allow() {
			fmt.Fprintln(c.conn, "Rate limit exceeded. Please wait.")
			return
		}
		s.outboundSpots <- sp
		fmt.Fprintln(c.conn, "Spot broadcast requested.")

	case cmd == "help":
		fmt.Fprintln(c.conn, "Commands: DX, #SPOT ON/OFF, BYE, HELP")

	case strings.HasPrefix(lower, "#spot"):
		if strings.Contains(lower, "on") {
			c.spotsOn = true
			fmt.Fprintln(c.conn, "Spots ON")
		} else {
			c.spotsOn = false
			fmt.Fprintln(c.conn, "Spots OFF")
		}

	case cmd == "bye" || cmd == "quit":
		fmt.Fprintln(c.conn, "Goodbye!")
		c.conn.Close()

	default:
		fmt.Fprintln(c.conn, "Unknown command.")
	}
}

func (s *Server) broadcastLoop() {
	for sp := range s.spotChan {
		// Deduplication
		if s.deduplicator.IsDuplicate(sp) {
			continue
		}

		s.cache.Add(sp)

		line := sp.String()
		s.mu.Lock()
		for _, client := range s.clients {
			if client.callsign != "" && client.spotsOn {
				fmt.Fprintln(client.conn, line)
			}
		}
		s.mu.Unlock()
	}
}
