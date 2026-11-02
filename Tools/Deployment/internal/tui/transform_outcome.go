package tui

// transform_outcome.go holds how the transform flow ends when the user cancels: Esc at a
// question, or declining the plan, shows a cancelled transform that wrote nothing.

import (
	"errors"

	"mosaic-deploy/internal/app"
)

// isRunCancellation reports whether err is a deliberate user abort of a run. For transform
// mode a declined plan is also a cancellation; the other modes restart their selection flow
// on a declined plan before this is consulted.
func isRunCancellation(err error) bool {
	return errors.Is(err, app.ErrSelectionCancelled) ||
		errors.Is(err, app.ErrModelSelectionCancelled) ||
		errors.Is(err, app.ErrPlanNotConfirmed)
}

// transformCancelledView is the done screen of a cancelled transform.
func (m *rootModel) transformCancelledView() string {
	return m.theme.Style(RoleWarning).Width(m.width).Render("Transform cancelled: nothing was written.") +
		"\n" + m.theme.Style(RoleHelp).Width(m.width).Render("press q or enter to exit")
}
