package runflow

import (
	"time"

	"mosaic-run/internal/domain"
)

// UpdateActivityStatus records the latest stream status of the current
// invocation and the time it was received; ignored when key is not current
// or the invocation has ended.
func (s *ProgressScreen) UpdateActivityStatus(key ActivityKey, st domain.ActivityStatus, at time.Time) {
	a := &s.activity
	if !a.begun || a.key != key || !a.endedAt.IsZero() {
		return
	}
	switch {
	case st.RunningToolID == "" && st.RunningTool == "":
		a.toolSince = time.Time{}
	case st.RunningToolID != a.status.RunningToolID || st.RunningTool != a.status.RunningTool || a.toolSince.IsZero():
		a.toolSince = at
	}
	a.status, a.statusAt = st, at
}
