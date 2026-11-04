package tui

// confighelp.go feeds the help overlay's "Configuration files" group: the pricing
// and analysis settings file of both levels and whether each exists.

import (
	tuicommon "mosaic-common/tui"

	"mosaic-log-analyzer/internal/domain"
)

// configFileLister is implemented by screens whose help overlay lists the
// configuration files.
type configFileLister interface {
	HelpOpen() bool
	SetConfigFiles([]tuicommon.HelpEntry)
}

// syncConfigHelp hands the open help overlay of the active screen the current
// configuration file entries; the files are only inspected while help is open.
func (m Model) syncConfigHelp() {
	if m.opts.Service == nil {
		return
	}
	var target configFileLister
	switch m.screen {
	case screenRuns:
		if m.runsScreen != nil {
			target = m.runsScreen
		}
	case screenPricing:
		if m.pricingScreen != nil {
			target = m.pricingScreen
		}
	default:
		target, _ = m.runView.(configFileLister)
	}
	if target == nil || !target.HelpOpen() {
		return
	}
	pricing, settings := m.opts.Service.PricingPaths(), m.opts.Service.SettingsPaths()
	target.SetConfigFiles([]tuicommon.HelpEntry{
		configFileEntry("pricing global", pricing.Global),
		configFileEntry("pricing project", pricing.Project),
		configFileEntry("settings global", settings.Global),
		configFileEntry("settings project", settings.Project),
	})
}

// configFileEntry describes one file: its path and (exists), (absent) or
// (unavailable) when the level has no file.
func configFileEntry(label string, f domain.ConfigFile) tuicommon.HelpEntry {
	switch {
	case f.Unavailable != "":
		return tuicommon.HelpEntry{Key: label, Desc: f.Unavailable + " (unavailable)"}
	case f.Exists:
		return tuicommon.HelpEntry{Key: label, Desc: f.Path + " (exists)"}
	default:
		return tuicommon.HelpEntry{Key: label, Desc: f.Path + " (absent)"}
	}
}
