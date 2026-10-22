package screens

// Key bindings that switch between the per-run views. The values must not
// collide with up/down/j/k/enter/esc/q/s/p.
const (
	KeyViewDispatches = "1" // run detail (the per-dispatch list)
	KeyViewOverview   = "2"
	KeyViewAgentTypes = "3"
	KeyViewStatus     = "4"
	KeyViewTools      = "5"
	KeyViewTimeline   = "6"
	KeyAcrossRuns     = "a" // on the runs list only
)

// RunView is satisfied by RunDetailScreen and every per-run view screen.
type RunView interface {
	Screen
	Back() bool                  // user asked to leave the per-run views
	Requested() (ScreenID, bool) // user pressed a view key for a different per-run view
	Reset()                      // clears Back and Requested
}
