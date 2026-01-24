package cluster

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valtss/spotmux/internal/metrics"
	"github.com/valtss/spotmux/internal/spot"
)

type UpstreamManager struct {
	callsign string
	name     string
	qth      string
	loc      string
	clusters []string
	spots    chan *spot.Spot
	conns    map[string]net.Conn
	mu       sync.Mutex
}

func NewUpstreamManager(callsign, name, qth, loc string, clusters []string) *UpstreamManager {
	return &UpstreamManager{
		callsign: callsign,
		name:     name,
		qth:      qth,
		loc:      loc,
		clusters: clusters,
		spots:    make(chan *spot.Spot, 100),
		conns:    make(map[string]net.Conn),
	}
}

func (m *UpstreamManager) Start() <-chan *spot.Spot {
	for _, addr := range m.clusters {
		go m.maintainConnection(addr)
	}
	return m.spots
}

func (m *UpstreamManager) maintainConnection(addr string) {
	backoff := time.Second
	for {
		err := m.connectAndRead(addr)
		if err != nil {
			atomic.AddUint64(&metrics.UpstreamErrors, 1)

			// Detect "Already Connected" conflict and use long backoff
			if strings.Contains(strings.ToLower(err.Error()), "already connected") ||
				strings.Contains(strings.ToLower(err.Error()), "already logged in") {
				log.Printf("Cluster %s: Conflict detected. Waiting 1 hour...", addr)
				backoff = 1 * time.Hour
			} else {
				log.Printf("Cluster %s connection error: %v. Retrying in %v...", addr, err, backoff)
				backoff *= 2
				if backoff > 5*time.Minute {
					backoff = 5 * time.Minute
				}
			}
		} else {
			// Successful connection closed without error (unlikely), reset backoff
			backoff = time.Second
		}

		// Connection lost or failed, remove from map
		m.mu.Lock()
		delete(m.conns, addr)
		m.mu.Unlock()

		time.Sleep(backoff)
	}
}

func (m *UpstreamManager) connectAndRead(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Handshake: Look for "login:" or "callsign:"
	// We read byte by byte because some clusters don't send a newline after the prompt.
	buffer := make([]byte, 0, 1024)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return err
		}
		buffer = append(buffer, b)
		lower := strings.ToLower(string(buffer))
		if strings.Contains(lower, "login:") || strings.Contains(lower, "callsign:") {
			fmt.Fprintf(conn, "%s\n", m.callsign)
			// Send profile settings if configured
			if m.name != "" {
				fmt.Fprintf(conn, "set/name %s\n", m.name)
			}
			if m.qth != "" {
				fmt.Fprintf(conn, "set/qth %s\n", m.qth)
			}
			if m.loc != "" {
				// Detection logic: Grid square is usually 4 or 6 chars, starts with 2 letters, then 2 digits, then optional 2 letters
				isGrid := false
				if len(m.loc) >= 4 && len(m.loc) <= 6 {
					isGrid = true
					for i, c := range m.loc {
						if (i < 2 && !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'))) ||
							(i >= 2 && i < 4 && !(c >= '0' && c <= '9')) {
							isGrid = false
							break
						}
					}
				}

				if isGrid {
					fmt.Fprintf(conn, "set/qra %s\n", m.loc)
				} else {
					fmt.Fprintf(conn, "set/location %s\n", m.loc)
				}
			}
			break
		}
		if len(buffer) > 1024 {
			// Prevent buffer from growing indefinitely if no prompt is found
			buffer = buffer[512:]
		}
	}

	// Handshake successful, track connection
	m.mu.Lock()
	m.conns[addr] = conn
	m.mu.Unlock()

	// Read loop
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		s, err := spot.ParseSpot(line, addr)
		if err == nil {
			m.spots <- s
		} else {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			lower := strings.ToLower(trimmed)
			// Check for conflict messages
			if strings.Contains(lower, "already connected") || strings.Contains(lower, "already logged in") {
				return fmt.Errorf("duplicate login: %s", trimmed)
			}
			// Log other non-spot messages
			log.Printf("[%s] %s", addr, trimmed)
		}
	}
}

func (m *UpstreamManager) BroadcastSpot(s *spot.Spot) {
	m.mu.Lock()
	defer m.mu.Unlock()

	line := fmt.Sprintf("dx %s %s %s\n", s.Frequency, s.DXCall, s.Remarks)
	for addr, conn := range m.conns {
		_, err := fmt.Fprint(conn, line)
		if err != nil {
			log.Printf("Failed to broadcast spot to %s: %v", addr, err)
			// Connection will be removed in maintainConnection read failure
		}
	}
}
