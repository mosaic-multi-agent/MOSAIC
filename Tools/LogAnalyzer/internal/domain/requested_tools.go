package domain

import (
	"encoding/json"
	"sort"
	"strconv"
)

// RequestedTools is the tools one API call requested (schema 1.7.0).
// The zero value is "not provided"; a provided value may be empty (no tools).
type RequestedTools struct {
	counts   map[string]int
	provided bool
}

// NewRequestedTools returns a provided value holding a copy of counts; entries with an
// empty name or a count below 1 are ignored. NewRequestedTools(nil) is provided and empty.
func NewRequestedTools(counts map[string]int) RequestedTools {
	kept := make(map[string]int, len(counts))
	for name, n := range counts {
		if name != "" && n >= 1 {
			kept[name] = n
		}
	}
	return RequestedTools{counts: kept, provided: true}
}

// IsProvided reports whether the harness provided the value.
func (r RequestedTools) IsProvided() bool { return r.provided }

// Total is the sum of the counts; 0 when empty or not provided.
func (r RequestedTools) Total() int {
	total := 0
	for _, n := range r.counts {
		total += n
	}
	return total
}

// Single returns the tool when the call requested exactly one tool once (Total() == 1).
func (r RequestedTools) Single() (tool string, ok bool) {
	if r.Total() != 1 {
		return "", false
	}
	for name := range r.counts {
		return name, true
	}
	return "", false
}

// Names returns the tool names sorted; nil when not provided.
func (r RequestedTools) Names() []string {
	if !r.provided {
		return nil
	}
	names := make([]string, 0, len(r.counts))
	for name := range r.counts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Count returns the count of one tool; 0 when not requested or not provided.
func (r RequestedTools) Count(tool string) int { return r.counts[tool] }

// Counts returns a copy; nil when not provided, non-nil empty when provided and empty.
func (r RequestedTools) Counts() map[string]int {
	if !r.provided {
		return nil
	}
	out := make(map[string]int, len(r.counts))
	for name, n := range r.counts {
		out[name] = n
	}
	return out
}

// Equal reports whether both values are provided-equal or both not provided.
func (r RequestedTools) Equal(o RequestedTools) bool {
	if r.provided != o.provided || len(r.counts) != len(o.counts) {
		return false
	}
	for name, n := range r.counts {
		if o.counts[name] != n {
			return false
		}
	}
	return true
}

// ParseRequestedTools reads a requested_tools JSON object. A missing key, null, a
// non-object, or any entry that is not a non-empty name with an integer count of at
// least 1 reads as not provided. {} is provided and empty.
func ParseRequestedTools(raw []byte) RequestedTools {
	if len(raw) == 0 {
		return RequestedTools{}
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
		return RequestedTools{}
	}
	counts := make(map[string]int, len(entries))
	for name, value := range entries {
		n, err := strconv.ParseInt(string(value), 10, 32)
		if name == "" || err != nil || n < 1 {
			return RequestedTools{}
		}
		counts[name] = int(n)
	}
	return NewRequestedTools(counts)
}

// MarshalJSON writes a provided value as an object (sorted names); a value that is
// not provided is written as null.
func (r RequestedTools) MarshalJSON() ([]byte, error) {
	if !r.provided {
		return []byte("null"), nil
	}
	return json.Marshal(r.counts)
}
