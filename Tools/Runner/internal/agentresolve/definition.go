package agentresolve

import (
	"errors"
	"fmt"
	"strings"

	"mosaic-common/agentload"
	"mosaic-common/mosaic"

	"mosaic-run/internal/agentfile"
	"mosaic-run/internal/domain"
)

// ErrDefinitionModelMissing: the definition's frontmatter has no model key,
// an empty or blank value, a non-scalar value, or no frontmatter at all.
var ErrDefinitionModelMissing = errors.New("agentresolve: agent definition declares no model")

// ErrDefinitionNameMismatch: the definition's frontmatter name is missing,
// empty, non-scalar, or differs from the agent identifier.
var ErrDefinitionNameMismatch = errors.New("agentresolve: agent definition name differs from its identifier")

// CheckDefinition loads ref.DefinitionPath through the shared agent loader and
// checks the frontmatter of its standard form. requireNameMatch adds the name
// check (Claude Code). A missing model is reported before a name mismatch.
// Every failure is a *domain.RefusalError naming the agent and its definition
// file; a file that cannot be loaded carries the *agentload.LoadError as cause.
func CheckDefinition(ref domain.AgentReference, f agentfile.Format, requireNameMatch bool) error {
	agent, err := agentload.Load(ref.DefinitionPath, f.FormatID)
	if err != nil {
		return definitionRefusal(ref, "cannot load agent file: "+err.Error(), err)
	}
	fm := agent.Document.Frontmatter()

	if scalarField(fm, "model") == "" {
		return definitionRefusal(ref, "frontmatter declares no model", ErrDefinitionModelMissing)
	}
	if requireNameMatch {
		if got := scalarField(fm, "name"); got != ref.Identifier {
			return definitionRefusal(ref,
				fmt.Sprintf("frontmatter name %q differs from the agent identifier %q", got, ref.Identifier),
				ErrDefinitionNameMismatch)
		}
	}
	return nil
}

// scalarField returns the trimmed scalar value of key, or "" when the key is
// absent, blank or not a scalar.
func scalarField(fm interface {
	Get(string) (mosaic.FieldValue, bool)
}, key string) string {
	fv, ok := fm.Get(key)
	if !ok || fv.Kind != mosaic.KindScalar {
		return ""
	}
	return strings.TrimSpace(fv.Scalar)
}

func definitionRefusal(ref domain.AgentReference, reason string, cause error) error {
	return &domain.RefusalError{
		Component: "agentresolve",
		Resource:  ref.Identifier,
		Reason:    "definition " + ref.DefinitionPath + ": " + reason,
		Cause:     cause,
		Section:   "frontmatter",
	}
}
