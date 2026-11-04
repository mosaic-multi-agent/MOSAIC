package domain

import (
	"fmt"
	"sort"
	"strings"
)

// EffortCount is the number of requests observed at one effort level.
type EffortCount struct {
	Level   string // verbatim effort value
	Records int    // requests at this level (superseded records counted once)
}

// EffortLevels lists the effort levels of one invocation's requests, ordered by
// Records descending then Level ascending. Empty means not provided.
type EffortLevels []EffortCount

// CountEffort counts each request with a non-empty Effort once, under its
// verbatim level. Requests without an effort are not counted.
func CountEffort(usages ...InvocationUsage) EffortLevels {
	counts := map[string]int{}
	for _, u := range usages {
		if u.Effort != "" {
			counts[u.Effort]++
		}
	}
	if len(counts) == 0 {
		return nil
	}
	out := make(EffortLevels, 0, len(counts))
	for level, n := range counts {
		out = append(out, EffortCount{Level: level, Records: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Records != out[j].Records {
			return out[i].Records > out[j].Records
		}
		return out[i].Level < out[j].Level
	})
	return out
}

// IsEmpty reports whether no request carried an effort level.
func (l EffortLevels) IsEmpty() bool {
	return len(l) == 0
}

// Label renders the levels: "" when empty, the bare level for one level,
// otherwise "<level> (<n>), <level> (<n>)..." in order.
func (l EffortLevels) Label() string {
	switch len(l) {
	case 0:
		return ""
	case 1:
		return l[0].Level
	}
	parts := make([]string, len(l))
	for i, c := range l {
		parts[i] = fmt.Sprintf("%s (%d)", c.Level, c.Records)
	}
	return strings.Join(parts, ", ")
}
