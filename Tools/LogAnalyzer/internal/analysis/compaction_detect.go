package analysis

import (
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

// defaultDropPercent applies when the settings carry no usable threshold, so a
// zero-valued settings value cannot turn every call pair into a detection.
const defaultDropPercent = 30

// detectedMarks finds the sharp context drops between consecutive calls of an
// actor. Calls without a context are skipped; a pair involving a partial call,
// or starting at a zero context, is never a detection.
func detectedMarks(calls []domain.ContextCall, dropPercent int) []domain.CompactionMark {
	if dropPercent < 1 {
		dropPercent = defaultDropPercent
	}
	var marks []domain.CompactionMark
	prev := -1
	for i, c := range calls {
		if !c.Context.IsPresent() {
			continue
		}
		if prev >= 0 {
			if mark, ok := dropBetween(calls, prev, i, dropPercent); ok {
				marks = append(marks, mark)
			}
		}
		prev = i
	}
	return marks
}

// dropBetween reports whether the context fell by at least dropPercent between
// calls[p] and calls[q], as a detected compaction mark.
func dropBetween(calls []domain.ContextCall, p, q, dropPercent int) (domain.CompactionMark, bool) {
	before, after := calls[p], calls[q]
	if before.Partial || after.Partial {
		return domain.CompactionMark{}, false
	}
	pTokens, qTokens := before.Context.ValueOr(0), after.Context.ValueOr(0)
	if pTokens <= 0 || (pTokens-qTokens)*100 < int64(dropPercent)*pTokens {
		return domain.CompactionMark{}, false
	}
	return domain.CompactionMark{
		Kind:         domain.CompactionDetected,
		At:           after.At,
		Before:       p,
		After:        q,
		BeforeTokens: pTokens,
		AfterTokens:  qTokens,
	}, true
}

// orderMarks sorts marks by time with zero times last; logged marks stay ahead
// of detected ones at the same time.
func orderMarks(marks []domain.CompactionMark) {
	sort.SliceStable(marks, func(i, j int) bool {
		a, b := marks[i].At, marks[j].At
		if a.IsZero() != b.IsZero() {
			return b.IsZero()
		}
		return a.Before(b)
	})
}
