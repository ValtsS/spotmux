package spot

import (
	"fmt"
	"strings"
	"time"
)

type Spot struct {
	Source     string    `json:"source"`
	Spotter    string    `json:"spotter"`
	Frequency  string    `json:"frequency"`
	DXCall     string    `json:"dx_call"`
	Remarks    string    `json:"remarks"`
	Time       string    `json:"time"`
	ReceivedAt time.Time `json:"received_at"`
}

// String returns the spot in a format suitable for sending to a DX cluster client
// Typical format: DX de K1ABC:     14025.0  KH6J         CW 599                 1234Z
func (s *Spot) String() string {
	return fmt.Sprintf("DX de %-10s %8s  %-12s %-25s %s",
		s.Spotter+":", s.Frequency, s.DXCall, s.Remarks, s.Time)
}

// ParseSpot takes a raw line from a DX cluster and attempts to parse it.
// Expected format is roughly: DX de K1ABC:     14025.0  KH6J         CW 599                 1234Z
func ParseSpot(line string, source string) (*Spot, error) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "DX de") {
		return nil, fmt.Errorf("not a spot line")
	}

	parts := strings.Fields(line)
	if len(parts) < 6 {
		return nil, fmt.Errorf("invalid spot line format")
	}

	// Basic parsing - this can be more robust with regex if needed
	// DX de K1ABC: 14025.0 KH6J CW 1234Z
	// 0  1  2      3       4    5  6

	spotter := strings.TrimSuffix(parts[2], ":")
	freq := parts[3]
	dxcall := parts[4]

	// Re-extracting remarks and time can be tricky because fields are variable
	// Let's use a more robust splitting for the tail

	// The last part is usually the time (e.g. 1234Z)
	timeStr := parts[len(parts)-1]

	// Everything between dxcall and time is remarks
	remarksStart := strings.Index(line, dxcall) + len(dxcall)
	remarksEnd := strings.LastIndex(line, timeStr)
	remarks := ""
	if remarksStart < remarksEnd {
		remarks = strings.TrimSpace(line[remarksStart:remarksEnd])
	}

	return &Spot{
		Source:     source,
		Spotter:    spotter,
		Frequency:  freq,
		DXCall:     dxcall,
		Remarks:    remarks,
		Time:       timeStr,
		ReceivedAt: time.Now(),
	}, nil
}
