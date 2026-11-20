package runflow

// SetGuidanceAvailable sets whether the session holds standing guidance, which
// decides whether the guidance key is offered and listed in the help bar.
func (s *ProgressScreen) SetGuidanceAvailable(available bool) { s.guidanceAvailable = available }

// GuidanceViewRequested reports whether the user asked to view and edit the
// standing guidance.
func (s *ProgressScreen) GuidanceViewRequested() bool { return s.guidanceView }

// ClearGuidanceViewRequest clears the request once the root model has acted on it.
func (s *ProgressScreen) ClearGuidanceViewRequest() { s.guidanceView = false }
