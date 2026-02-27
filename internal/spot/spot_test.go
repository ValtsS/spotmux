package spot

import "testing"

func TestShouldIgnore(t *testing.T) {
	prefixes := []string{"R", "UA", "UB", "EU", "EW", "UA2", "RA2"}
	filter := NewPrefixFilter(prefixes)

	tests := []struct {
		callsign string
		ignored  bool
	}{
		{"UA1ABC", true},
		{"R1ABC", true},
		{"EU1A", true},
		{"EW1A", true},
		{"UA2F", true},
		{"RA2F", true},
		{"UB1A", true},
		{"YL3OI", false},
		{"K1ABC", false},
		{"G4ABC", false},
		{"LZ1ABC", false},
		{"", false},
		{"U", false}, // Only UA matches UA prefix
	}

	for _, tt := range tests {
		s := &Spot{DXCall: tt.callsign}
		got := s.ShouldIgnore(filter)
		if got != tt.ignored {
			t.Errorf("ShouldIgnore(%q) = %v; want %v", tt.callsign, got, tt.ignored)
		}
	}
}

func TestShouldIgnoreMixedCase(t *testing.T) {
	prefixes := []string{"ua", "Eu"}
	filter := NewPrefixFilter(prefixes)
	s := &Spot{DXCall: "UA1ABC"}
	if !s.ShouldIgnore(filter) {
		t.Errorf("ShouldIgnore should be case-insensitive for prefixes")
	}

	s = &Spot{DXCall: "eu1a"}
	if !s.ShouldIgnore(filter) {
		t.Errorf("ShouldIgnore should be case-insensitive for callsigns")
	}
}
