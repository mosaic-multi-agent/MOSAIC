package transform_test

// owned_content_test.go specifies OwnedContentOf and DiffRegions: the Deploy-owned content
// of an agent is its body without project-region content and custom regions; frontmatter is
// not part of it; a skill's is the whole file.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

func TestOwnedContentOf_Agent_HashIsCanonicalForm(t *testing.T) {
	got := mustOwnedAgent(t, ownedAgentLF)

	hex := strings.TrimPrefix(got.Hash, "sha256:")
	if !strings.HasPrefix(got.Hash, "sha256:") || len(hex) != 64 || hex != strings.ToLower(hex) {
		t.Errorf("Hash = %q; want sha256:<64 lowercase hex>", got.Hash)
	}
}

func TestOwnedContentOf_Agent_SameInputSameRecord(t *testing.T) {
	a := mustOwnedAgent(t, ownedAgentLF)
	b := mustOwnedAgent(t, ownedAgentLF)

	if !reflect.DeepEqual(a, b) {
		t.Errorf("records differ for identical input:\n%+v\n%+v", a, b)
	}
}

func TestOwnedContentOf_Agent_UnchangedByEditsOutsideDeployOwnedContent(t *testing.T) {
	base := mustOwnedAgent(t, ownedAgentLF)
	cases := []struct{ name, old, repl string }{
		{"project region filled", "<IdentityExtension type=\"project\">\n", "<IdentityExtension type=\"project\">\nTeam context written by hand.\n"},
		{"custom region filled", "<TeamNotes type=\"custom\">\n", "<TeamNotes type=\"custom\">\nOur own rule.\n"},
		{"custom region removed with its tags", "<TeamNotes type=\"custom\">\n</TeamNotes>\n", ""},
		{"frontmatter version changed", "version: 1.0.0", "version: 9.9.9"},
		{"frontmatter tools changed", "tools: [file_read]", "tools: [file_read, bash]"},
		{"frontmatter key added", "name: owned-agent", "name: owned-agent\nextra_key: user value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustOwnedAgent(t, replaceOnce(t, ownedAgentLF, tc.old, tc.repl))

			if !reflect.DeepEqual(got, base) {
				t.Errorf("record changed by an edit that is not Deploy-owned:\ngot:  %+v\nwant: %+v", got, base)
			}
		})
	}
}

func TestOwnedContentOf_Agent_ChangedByEditsOfDeployOwnedContent(t *testing.T) {
	base := mustOwnedAgent(t, ownedAgentLF)
	cases := []struct{ name, old, repl string }{
		{"text outside regions", "Outside prose that Deploy owns.", "Outside prose edited."},
		{"core prose of a region", "Identity core prose.", "Identity core prose edited."},
		{"managed child region", "Harness constraint text.", "Harness constraint text edited."},
		{"project region boundary tag", "<IdentityExtension type=\"project\">", "<IdentityExtension type=\"project\" note=\"x\">"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustOwnedAgent(t, replaceOnce(t, ownedAgentLF, tc.old, tc.repl))

			if got.Hash == base.Hash {
				t.Errorf("Hash unchanged (%s) although Deploy-owned content was edited", got.Hash)
			}
		})
	}
}

func TestOwnedContentOf_Agent_BOMAndLineEndingsDoNotChangeRecord(t *testing.T) {
	base := mustOwnedAgent(t, ownedAgentLF)
	crlf := crlfBytes(ownedAgentLF)
	cases := []struct {
		name string
		in   []byte
	}{
		{"CRLF", crlf},
		{"BOM", withBOM([]byte(ownedAgentLF))},
		{"BOM and CRLF", withBOM(crlf)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustOwned(t, domain.ArtifactAgent, tc.in)

			if !reflect.DeepEqual(got, base) {
				t.Errorf("record differs from the LF form:\ngot:  %+v\nwant: %+v", got, base)
			}
		})
	}
}

func TestOwnedContentOf_Agent_RegionsListTopLevelRegionsInDocumentOrder(t *testing.T) {
	got := mustOwnedAgent(t, ownedAgentLF)

	var names []string
	for _, r := range got.Regions {
		names = append(names, r.Name)
		if !strings.HasPrefix(r.Hash, "sha256:") {
			t.Errorf("region %q hash = %q; want sha256: form", r.Name, r.Hash)
		}
	}
	want := []string{domain.OutsideRegionsName, "Identity", "Constraints"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("region names = %v; want %v", names, want)
	}
}

func TestOwnedContentOf_Agent_EditChangesOnlyTheEnclosingRegionHash(t *testing.T) {
	base := mustOwnedAgent(t, ownedAgentLF)
	cases := []struct{ name, old, repl, region string }{
		{"outside text", "Outside prose that Deploy owns.", "Outside prose edited.", domain.OutsideRegionsName},
		{"region core prose", "Identity core prose.", "Identity core prose edited.", "Identity"},
		{"nested managed region", "Harness constraint text.", "Harness constraint text edited.", "Constraints"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustOwnedAgent(t, replaceOnce(t, ownedAgentLF, tc.old, tc.repl))

			if len(got.Regions) != len(base.Regions) {
				t.Fatalf("region count = %d; want %d", len(got.Regions), len(base.Regions))
			}
			for i := range got.Regions {
				differs := got.Regions[i].Hash != base.Regions[i].Hash
				if differs != (got.Regions[i].Name == tc.region) {
					t.Errorf("region %q hash differs = %v; edited region is %q", got.Regions[i].Name, differs, tc.region)
				}
			}
		})
	}
}

func TestOwnedContentOf_Agent_ProjectRegionInsideCustomRegionContentIsExcluded(t *testing.T) {
	const emptyNested = "<TeamNotes type=\"custom\">\n<Inner type=\"project\">\n</Inner>\n</TeamNotes>"
	const filledNested = "<TeamNotes type=\"custom\">\n<Inner type=\"project\">\nDeep text.\n</Inner>\n</TeamNotes>"
	empty := mustOwnedAgent(t, replaceOnce(t, ownedAgentLF, "<TeamNotes type=\"custom\">\n</TeamNotes>", emptyNested))
	filled := mustOwnedAgent(t, replaceOnce(t, ownedAgentLF, "<TeamNotes type=\"custom\">\n</TeamNotes>", filledNested))

	if !reflect.DeepEqual(empty, filled) {
		t.Errorf("project content inside a custom region changed the record:\n%+v\n%+v", empty, filled)
	}
}

func TestOwnedContentOf_Agent_TopLevelProjectRegionFillDoesNotChangeRecord(t *testing.T) {
	const withEmpty = ownedAgentLF + "\n<Notes type=\"project\">\n</Notes>\n"
	const withFilled = ownedAgentLF + "\n<Notes type=\"project\">\nFilled by hand.\n</Notes>\n"

	empty := mustOwnedAgent(t, withEmpty)
	filled := mustOwnedAgent(t, withFilled)

	if !reflect.DeepEqual(empty, filled) {
		t.Errorf("filling a top-level project region changed the record:\n%+v\n%+v", empty, filled)
	}
}

func TestOwnedContentOf_Agent_BodyWithoutRegionsOrEmptyBodyIsValid(t *testing.T) {
	for name, text := range map[string]string{
		"prose only": "---\nname: plain\n---\n\nJust prose.\n",
		"empty body": "---\nname: plain\n---\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := mustOwnedAgent(t, text)

			if !strings.HasPrefix(got.Hash, "sha256:") {
				t.Errorf("Hash = %q; want sha256: form", got.Hash)
			}
			for _, r := range got.Regions {
				if r.Name != domain.OutsideRegionsName {
					t.Errorf("region %q listed for a body without regions", r.Name)
				}
			}
		})
	}
}

func TestOwnedContentOf_Agent_ProseOnlyBodyEditChangesRecord(t *testing.T) {
	a := mustOwnedAgent(t, "---\nname: plain\n---\n\nJust prose.\n")
	b := mustOwnedAgent(t, "---\nname: plain\n---\n\nJust prose edited.\n")

	if a.Hash == b.Hash {
		t.Errorf("edit of a body without regions left Hash unchanged (%s)", a.Hash)
	}
}

func TestOwnedContentOf_Agent_UnclosedRegionTagIsAnError(t *testing.T) {
	// The well-formed form must compute, so an implementation that errors for every agent fails.
	mustOwnedAgent(t, ownedAgentLF)
	broken := replaceOnce(t, ownedAgentLF, "</Identity>\n", "")

	if _, err := transform.OwnedContentOf(domain.ArtifactAgent, []byte(broken)); err == nil {
		t.Error("OwnedContentOf of an agent with an unclosed region tag returned no error")
	}
}

func TestOwnedContentOf_Skill_WholeFileIsOwned(t *testing.T) {
	const skill = "---\nname: demo\nversion: 1.0\n---\n\nSkill body.\n"
	base := mustOwned(t, domain.ArtifactSkill, []byte(skill))
	cases := []struct{ name, text string }{
		{"frontmatter edit", strings.Replace(skill, "version: 1.0", "version: 2.0", 1)},
		{"body edit", strings.Replace(skill, "Skill body.", "Skill body edited.", 1)},
		{"appended line", skill + "extra\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustOwned(t, domain.ArtifactSkill, []byte(tc.text))

			if got.Hash == base.Hash {
				t.Errorf("skill record unchanged by %s", tc.name)
			}
		})
	}
}

func TestOwnedContentOf_Skill_HasNoRegionsAndIgnoresBOMAndLineEndings(t *testing.T) {
	const skill = "---\nname: demo\n---\n\nSkill body.\n"
	base := mustOwned(t, domain.ArtifactSkill, []byte(skill))

	if base.Regions != nil {
		t.Errorf("skill Regions = %v; want nil", base.Regions)
	}
	for name, in := range map[string][]byte{"CRLF": crlfBytes(skill), "BOM": withBOM([]byte(skill))} {
		if got := mustOwned(t, domain.ArtifactSkill, in); got.Hash != base.Hash {
			t.Errorf("%s: Hash = %s; want %s", name, got.Hash, base.Hash)
		}
	}
}

func TestDiffRegions_NamesChangedAndOneSidedRegionsOnce(t *testing.T) {
	rec := domain.OwnedContent{Regions: []domain.RegionHash{{Name: "A", Hash: "h1"}, {Name: "B", Hash: "h2"}, {Name: "Gone", Hash: "h3"}}}
	cur := domain.OwnedContent{Regions: []domain.RegionHash{{Name: "A", Hash: "h1"}, {Name: "B", Hash: "other"}, {Name: "New", Hash: "h4"}}}

	got := transform.DiffRegions(rec, cur)

	want := []string{"B", "Gone", "New"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DiffRegions = %v; want %v (recorded order, then current order)", got, want)
	}
}

func TestDiffRegions_IdenticalRegionsGiveNone(t *testing.T) {
	r := domain.OwnedContent{Regions: []domain.RegionHash{{Name: "A", Hash: "h1"}}}

	if got := transform.DiffRegions(r, r); len(got) != 0 {
		t.Errorf("DiffRegions of equal records = %v; want none", got)
	}
}

func TestDiffRegions_EmptyWhenEitherSideHasNoRegions(t *testing.T) {
	with := domain.OwnedContent{Regions: []domain.RegionHash{{Name: "A", Hash: "h1"}}}
	without := domain.OwnedContent{Hash: "x"}

	if got := transform.DiffRegions(with, without); len(got) != 0 {
		t.Errorf("DiffRegions(with, without) = %v; want none", got)
	}
	if got := transform.DiffRegions(without, with); len(got) != 0 {
		t.Errorf("DiffRegions(without, with) = %v; want none", got)
	}
}
