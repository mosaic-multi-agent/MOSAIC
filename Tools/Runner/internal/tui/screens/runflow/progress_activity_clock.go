package runflow

import "time"

// SetClock sets the time source used for activity start/end stamps and for
// rendering durations; nil restores time.Now.
func (s *ProgressScreen) SetClock(now func() time.Time) { s.clock = now }

// activityNow is the current time of the activity panel.
func (s *ProgressScreen) activityNow() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}
