package agentresolve_test

// Tests for agentresolve.CheckDefinition reading through the shared agent
// loader: a decodable definition of any registered format is checked on its
// standard form, and a file that cannot be loaded refuses naming the agent, the
// file and the reason.

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"mosaic-common/agentload"

	"mosaic-run/internal/agentfile"
	"mosaic-run/internal/agentresolve"
	"mosaic-run/internal/domain"
)

func jsonDefinitionRef(name string) domain.AgentReference {
	return domain.AgentReference{Identifier: checkedAgentID, DefinitionPath: testJSONDefinition(name)}
}

func TestCheckDefinition_NonMarkdownFile_WithModel_Passes(t *testing.T) {
	f := useTestJSON(t)

	err := agentresolve.CheckDefinition(jsonDefinitionRef("with-model.json"), f, false)

	if err != nil {
		t.Errorf("want nil for a decodable definition with a model, got %v", err)
	}
}

func TestCheckDefinition_NonMarkdownFile_WithModelAndMatchingName_PassesNameCheck(t *testing.T) {
	f := useTestJSON(t)

	err := agentresolve.CheckDefinition(jsonDefinitionRef("with-model.json"), f, true)

	if err != nil {
		t.Errorf("want nil when the name equals the identifier, got %v", err)
	}
}

func TestCheckDefinition_NonMarkdownFile_WithoutModel_RefusesAsMissingModel(t *testing.T) {
	f := useTestJSON(t)
	ref := jsonDefinitionRef("no-model.json")

	err := agentresolve.CheckDefinition(ref, f, false)

	if !errors.Is(err, agentresolve.ErrDefinitionModelMissing) {
		t.Fatalf("want ErrDefinitionModelMissing, got %v", err)
	}
	requireRefusalNames(t, err, ref)
}

func TestCheckDefinition_NonMarkdownFile_NameMismatch_RefusesWhenNameCheckRequested(t *testing.T) {
	f := useTestJSON(t)
	ref := jsonDefinitionRef("other-name.json")

	err := agentresolve.CheckDefinition(ref, f, true)

	if !errors.Is(err, agentresolve.ErrDefinitionNameMismatch) {
		t.Fatalf("want ErrDefinitionNameMismatch, got %v", err)
	}
	requireRefusalNames(t, err, ref)
}

func TestCheckDefinition_NonMarkdownFile_NameMismatch_PassesWithoutNameCheck(t *testing.T) {
	f := useTestJSON(t)

	err := agentresolve.CheckDefinition(jsonDefinitionRef("other-name.json"), f, false)

	if err != nil {
		t.Errorf("want nil when the name check is off, got %v", err)
	}
}

func TestCheckDefinition_UndecodableFile_RefusesNamingAgentFileAndReason(t *testing.T) {
	f := useTestJSON(t)
	ref := jsonDefinitionRef("undecodable.json")

	err := agentresolve.CheckDefinition(ref, f, false)

	if err == nil {
		t.Fatal("want a refusal for a file that cannot be decoded, got nil")
	}
	re := requireRefusalNames(t, err, ref)
	if !errors.Is(err, agentload.ErrUndecodable) {
		t.Errorf("want the cause to match agentload.ErrUndecodable, got %v", err)
	}
	var loadErr *agentload.LoadError
	if !errors.As(re.Cause, &loadErr) {
		t.Errorf("want the refusal's Cause to be a *agentload.LoadError, got %T", re.Cause)
	}
	if errors.Is(err, agentresolve.ErrDefinitionModelMissing) || errors.Is(err, agentresolve.ErrDefinitionNameMismatch) {
		t.Errorf("a decode failure is neither a missing model nor a name mismatch: %v", err)
	}
}

func TestCheckDefinition_MissingFile_RefusesAsUnreadable(t *testing.T) {
	f := useTestJSON(t)
	ref := domain.AgentReference{
		Identifier:     checkedAgentID,
		DefinitionPath: filepath.Join(t.TempDir(), "gone", checkedAgentID+".json"),
	}

	err := agentresolve.CheckDefinition(ref, f, false)

	if !errors.Is(err, agentload.ErrUnreadable) {
		t.Errorf("want the cause to match agentload.ErrUnreadable, got %v", err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("want the read error to stay reachable, got %v", err)
	}
	requireRefusalNames(t, err, ref)
}

func TestCheckDefinition_UnknownFormat_RefusesNamingAgentAndFile(t *testing.T) {
	ref := jsonDefinitionRef("with-model.json")
	unknown := agentfile.Format{FormatID: "no-such-format", Extension: ".json"}

	err := agentresolve.CheckDefinition(ref, unknown, false)

	if !errors.Is(err, agentload.ErrUnknownFormat) {
		t.Errorf("want the cause to match agentload.ErrUnknownFormat, got %v", err)
	}
	requireRefusalNames(t, err, ref)
}

func TestCheckDefinition_MarkdownMalformedFrontmatter_RefusesAsUndecodable(t *testing.T) {
	ref := writeDefinition(t, checkedAgentID, "---\nname: "+checkedAgentID+"\nmodel: [unclosed\n---\nbody\n")

	err := agentresolve.CheckDefinition(ref, legacyMarkdown, true)

	if !errors.Is(err, agentload.ErrUndecodable) {
		t.Errorf("want the cause to match agentload.ErrUndecodable, got %v", err)
	}
	requireRefusalNames(t, err, ref)
}

func TestCheckDefinition_MarkdownMissingFile_RefusesAsUnreadable(t *testing.T) {
	ref := domain.AgentReference{
		Identifier:     checkedAgentID,
		DefinitionPath: filepath.Join(t.TempDir(), "gone", checkedAgentID+".md"),
	}

	err := agentresolve.CheckDefinition(ref, legacyMarkdown, false)

	if !errors.Is(err, agentload.ErrUnreadable) {
		t.Errorf("want the cause to match agentload.ErrUnreadable, got %v", err)
	}
	requireRefusalNames(t, err, ref)
}
