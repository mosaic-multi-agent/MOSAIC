package runner

import (
	"fmt"
	"strings"
)

// DefinitionModelProblem classifies a pre-spawn definition failure.
type DefinitionModelProblem string

const (
	DefinitionModelMissing     DefinitionModelProblem = "missing"     // key absent or value empty
	DefinitionModelMismatch    DefinitionModelProblem = "mismatch"    // value != expected
	DefinitionModelUnreadable  DefinitionModelProblem = "unreadable"  // reader returned an error
	DefinitionModelUnsupported DefinitionModelProblem = "unsupported" // adapter is no DefinitionModelReader

	// Fail-closed tier resolution on the catalogue path.
	DefinitionModelNoCatalogEntry        DefinitionModelProblem = "no catalog entry"              // key matches no test catalog file
	DefinitionModelAmbiguousCatalogEntry DefinitionModelProblem = "ambiguous catalog entry"       // key matches 2+ test catalog files
	DefinitionModelNoDeclaredTier        DefinitionModelProblem = "no declared tier"              // catalog file declares no recommended_tier
	DefinitionModelUnselectedTier        DefinitionModelProblem = "tier without a selected model" // tier has no non-empty TierModels() entry
)

// DefinitionModelError is the setup error Run returns when a deployed definition
// does not carry the model selected for the tier its agent declares, or when
// that tier cannot be resolved (fail closed).
type DefinitionModelError struct {
	Agent         string   // deployed/rendered agent key
	Path          string   // absolute path of the offending definition
	Tier          string   // declared (or attributed) tier; "" when not known
	Expected      string   // the model selected for Tier; "" when not known
	Found         string   // the parsed value; "" when missing
	CatalogFolder string   // test catalog folder the tier was read from
	CatalogFiles  []string // catalog file(s) the agent key joined to
	Problem       DefinitionModelProblem
}

// Error: "pre-spawn check: agent <agent>: deployed definition <path>: <problem>: <detail>".
// For definition read-back problems the detail names the declared tier, the
// model expected for it and the model found; for the fail-closed tier
// resolution problems it names the situation with the catalog folder or files
// and whichever of tier and model are known.
func (e *DefinitionModelError) Error() string {
	return fmt.Sprintf("pre-spawn check: agent %s: deployed definition %s: %s: %s",
		e.Agent, e.Path, e.Problem, e.detail())
}

func (e *DefinitionModelError) detail() string {
	switch e.Problem {
	case DefinitionModelNoCatalogEntry:
		if e.CatalogFolder == "" {
			return "no test catalog folder configured"
		}
		return fmt.Sprintf("no agent file for key %s in test catalog %s", e.Agent, e.CatalogFolder)
	case DefinitionModelAmbiguousCatalogEntry:
		return fmt.Sprintf("key %s matches catalog files %s", e.Agent, strings.Join(e.CatalogFiles, ", "))
	case DefinitionModelNoDeclaredTier:
		return fmt.Sprintf("catalog file %s declares no recommended_tier", e.firstCatalogFile())
	case DefinitionModelUnselectedTier:
		return fmt.Sprintf("tier %s declared by %s has no selected model", e.Tier, e.firstCatalogFile())
	}
	found := e.Found
	if found == "" {
		found = "none"
	}
	return fmt.Sprintf("tier %s, expected model %s, found %s", e.Tier, e.Expected, found)
}

func (e *DefinitionModelError) firstCatalogFile() string {
	if len(e.CatalogFiles) == 0 {
		return ""
	}
	return e.CatalogFiles[0]
}
