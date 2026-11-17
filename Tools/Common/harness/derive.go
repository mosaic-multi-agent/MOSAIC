package harness

import (
	"fmt"

	"mosaic-common/harness/builtindesc"
)

// loadCatalogs derives both catalogs from the embedded builtin descriptors, in
// descriptor order. A descriptor that fails validation is a build defect that
// Common's tests catch before release, so it panics at package init.
func loadCatalogs() ([]CLIHarness, []ModelCatalog) {
	descs, err := builtindesc.Builtins()
	if err != nil {
		panic(fmt.Sprintf("harness: invalid builtin descriptor: %v", err))
	}
	var clis []CLIHarness
	var models []ModelCatalog
	for _, d := range descs {
		if d.Models != nil {
			models = append(models, ModelCatalog{
				HarnessID:  d.ID,
				IDs:        copyIDs(d.Models.IDs),
				FormatHint: d.Models.FormatHint,
			})
		}
		if d.CLI == nil {
			continue
		}
		clis = append(clis, CLIHarness{
			ID:                 d.ID,
			Label:              d.CLI.Label,
			AgentsDir:          d.AgentsDir,
			LoadingMechanism:   loadingMechanismOf(d.CLI.LoadingMechanism),
			AgentFormatID:      d.AgentFormatID,
			AgentExtension:     d.AgentExtension,
			SkillsDir:          d.SkillsDir,
			HooksDir:           d.HooksDir,
			ExperimentalReason: d.ExperimentalReason,
		})
	}
	return clis, models
}

// loadingMechanismOf maps a descriptor's cli.loading_mechanism value, already
// validated by the descriptor package, to its LoadingMechanism.
func loadingMechanismOf(wire string) LoadingMechanism {
	switch wire {
	case builtindesc.LoadingMechanismName:
		return LoadingMechanismName
	case builtindesc.LoadingMechanismRunnerLoaded:
		return LoadingMechanismRunnerLoaded
	default:
		return LoadingMechanismUnset
	}
}
