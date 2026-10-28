package tui

// The styles handed to the screens carry the palette's signal and data roles.

import (
	"testing"

	tuicommon "mosaic-common/tui"
)

func TestStylesFromTheme_CarryEverySignalAndDataRole(t *testing.T) {
	st := stylesFromTheme(tuicommon.DefaultTheme())

	roles := []tuicommon.StyleRole{
		tuicommon.RoleSignalGood, tuicommon.RoleSignalWarning, tuicommon.RoleSignalBad, tuicommon.RoleSignalMuted,
		tuicommon.RoleDataCost, tuicommon.RoleDataTokens, tuicommon.RoleDataTime, tuicommon.RoleDataActor,
		tuicommon.RoleDataLead, tuicommon.RoleDataTool, tuicommon.RoleDataModel,
	}
	for _, role := range roles {
		if _, ok := st.Theme.Styles[role]; !ok {
			t.Errorf("styles passed to screens lack role %q", role)
		}
	}
}

func TestStylesFromTheme_KeepsTheExistingRoleStyles(t *testing.T) {
	theme := tuicommon.DefaultTheme()

	st := stylesFromTheme(theme)

	if st.Title.Render("x") != theme.Style(tuicommon.RoleTitle).Render("x") ||
		st.Warning.Render("x") != theme.Style(tuicommon.RoleWarning).Render("x") ||
		st.Border.Render("x") != theme.Style(tuicommon.RoleBorder).Render("x") {
		t.Error("the existing role styles must be unchanged")
	}
	for _, role := range []tuicommon.StyleRole{tuicommon.RoleTitle, tuicommon.RoleHelp, tuicommon.RoleSelected} {
		if _, ok := st.Theme.Styles[role]; !ok {
			t.Errorf("styles passed to screens lack the existing role %q", role)
		}
	}
}
