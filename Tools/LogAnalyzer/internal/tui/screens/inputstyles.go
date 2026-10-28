package screens

import (
	tuicommon "mosaic-common/tui"
	"mosaic-common/tui/widgets"
)

// textInputStyles are the styles of a page's text input: the label as a
// subtitle and a validation message in the bad signal role.
func textInputStyles(st Styles) widgets.TextInputStyles {
	return widgets.TextInputStyles{
		Label:  st.Subtitle,
		Input:  st.Body,
		ErrMsg: st.Theme.Style(tuicommon.RoleSignalBad),
	}
}
