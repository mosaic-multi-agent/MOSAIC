package runflow

// guardOffHelpText is the done-screen help entry for resuming with the guard off.
const guardOffHelpText = "g  continue with loop guard off"

// ContinueWithGuardOff reports whether the user chose to resume with the
// anti-loop guard off. Only possible when the outcome's AntiLoopStop is true.
func (s *DoneScreen) ContinueWithGuardOff() bool { return s.contGuardOff }
