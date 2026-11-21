package screens

// harness_config.go declares the contract of the configurable harness screen: the role wording,
// the configuration value and the mode-to-role mapping.

import (
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/tui/widgets"
)

// HarnessRole names what the chosen harness is for; it selects the harness screen's title
// and subtitle.
type HarnessRole int

const (
	HarnessRoleNone            HarnessRole = iota // today's wording; used before Configure
	HarnessRoleDeployTarget                       // Deploy workspace, Deploy agents, Deploy hooks
	HarnessRoleDeployment                         // Update workspace, Change workflows
	HarnessRolePromoteSource                      // Promote to generic
	HarnessRoleTransformSource                    // Transform harness, source screen
	HarnessRoleTransformTarget                    // Transform harness, target screen
)

// HarnessScreenConfig is what HarnessScreen.Configure shows.
type HarnessScreenConfig struct {
	Role           HarnessRole
	Harnesses      []domain.HarnessRef
	SourceName     string
	DisabledID     string
	DisabledReason string
	HighlightID    string
}

// HarnessRoleForMode returns the role the harness plays on the first harness screen of mode:
// DeployTarget for the three Deploy modes, Deployment for Update workspace and Change
// workflows, PromoteSource for Promote to generic, TransformSource for Transform harness,
// HarnessRoleNone for an unknown mode. It never returns HarnessRoleTransformTarget.
func HarnessRoleForMode(mode domain.RunMode) HarnessRole {
	switch mode {
	case domain.ModeDeployWorkspace, domain.ModeDeployAgents, domain.ModeDeployHooks:
		return HarnessRoleDeployTarget
	case domain.ModeUpdateWorkspace, domain.ModeUpdateWorkflows:
		return HarnessRoleDeployment
	case domain.ModePromoteToGeneric:
		return HarnessRolePromoteSource
	case domain.ModeTransformHarness:
		return HarnessRoleTransformSource
	}
	return HarnessRoleNone
}

// harnessWording returns the title and subtitle for role. sourceName only matters for
// HarnessRoleTransformTarget.
func harnessWording(role HarnessRole, sourceName string) (title, subtitle string) {
	switch role {
	case HarnessRoleDeployTarget:
		return "Select Target Harness", "The harness to deploy to."
	case HarnessRoleDeployment:
		return "Select Deployed Harness",
			"The harness whose deployment in this workspace is updated or changed."
	case HarnessRolePromoteSource:
		return "Select Source Harness",
			"The harness the agents are written for. They become generic sources in a catalog."
	case HarnessRoleTransformSource:
		return "Select Source Harness", "The harness to transform from."
	case HarnessRoleTransformTarget:
		if sourceName == "" {
			return "Select Target Harness", "The harness to transform to."
		}
		return "Select Target Harness", "The harness to transform to, from " + sourceName + "."
	}
	return "Select Harness", ""
}

// Configure replaces what the screen shows: the role wording, the offered harnesses, at most
// one harness disabled with a given reason, and the highlighted harness. It clears the Done
// and Back flags and refreshes the detail pane.
func (s *HarnessScreen) Configure(cfg HarnessScreenConfig) {
	s.role = cfg.Role
	s.sourceName = cfg.SourceName
	s.harnesses = cfg.Harnesses

	items := makeHarnessItems(cfg.Harnesses, s.styles)
	if cfg.DisabledID != "" {
		for i := range items {
			if items[i].ID == cfg.DisabledID {
				items[i].Disabled = true
				items[i].DisabledReason = cfg.DisabledReason
			}
		}
	}
	listHeight, listWidth, _ := harnessLayout(s.width, s.height)
	// NewList starts on the first enabled item, or the first item when none is enabled.
	s.list = widgets.NewList(items, listHeight, listWidth, harnessListStyles(s.styles))
	for _, it := range items {
		if it.ID == cfg.HighlightID && !it.Disabled {
			s.list.SetCursor(it.ID)
			break
		}
	}
	s.updateDetail()
}
