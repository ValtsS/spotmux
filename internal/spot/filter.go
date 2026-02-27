package spot

import "strings"

// PrefixFilter stores prefixes grouped by length for efficient lookup
type PrefixFilter struct {
	buckets map[int]map[string]struct{}
	lengths []int
}

// NewPrefixFilter creates a new PrefixFilter from a list of prefixes
func NewPrefixFilter(prefixes []string) *PrefixFilter {
	f := &PrefixFilter{
		buckets: make(map[int]map[string]struct{}),
		lengths: []int{},
	}

	for _, p := range prefixes {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		l := len(p)
		if _, ok := f.buckets[l]; !ok {
			f.buckets[l] = make(map[string]struct{})
			f.lengths = append(f.lengths, l)
		}
		f.buckets[l][p] = struct{}{}
	}
	return f
}

// Matches returns true if the callsign starts with any of the ignored prefixes
func (f *PrefixFilter) Matches(callsign string) bool {
	if f == nil || len(f.buckets) == 0 {
		return false
	}
	callsign = strings.ToUpper(callsign)
	for _, l := range f.lengths {
		if len(callsign) < l {
			continue
		}
		prefix := callsign[:l]
		if _, ok := f.buckets[l][prefix]; ok {
			return true
		}
	}
	return false
}
