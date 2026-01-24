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
	clusters []string
	spots    chan *spot.Spot
	conns    map[string]net.Conn
	mu       sync.Mutex
}

func NewUpstreamManager(callsign string, clusters []string) *UpstreamManager {
	return &UpstreamManager{
		callsign: callsign,
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
			log.Printf("Cluster %s connection error: %v. Retrying in %v...", addr, err, backoff)
		}
		// Connection lost or failed, remove from map
		m.mu.Lock()
		delete(m.conns, addr)
		m.mu.Unlock()

		time.Sleep(backoff)
		backoff *= 2
		if backoff > 5*time.Minute {
			backoff = 5 * time.Minute
		}
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
