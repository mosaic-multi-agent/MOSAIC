package app

// owned_content_canonical_probe_test.go specifies that the probe computes the Deploy-owned
// content record of an agent from the agent's canonical (decoded) form, so edit detection
// does not depend on the file format: frontmatter and carried keys never affect the record,
// while body edits and Deploy-owned region edits do and project and custom region edits do
// not. For a Markdown file the decode is the identity, so Markdown records are unchanged.

import (
	"strings"
	"testing"

	"mosaic-deploy/internal/agentformat"
	_ "mosaic-deploy/internal/agentformat/all"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// cxOwnedBody is a canonical agent body: text outside any region, a core region holding a
// project region, and a core region holding a custom region.
const cxOwnedBody = "Outside prose.\n\n" +
	"<Identity type=\"core\">\nIdentity prose.\n<IdentityExtension type=\"project\">\n</IdentityExtension>\n</Identity>\n\n" +
	"<Constraints type=\"core\">\nConstraint prose.\n<TeamNotes type=\"custom\">\n</TeamNotes>\n</Constraints>\n"

// cxOwnedOpts shapes one Codex agent file of the probe tests.
type cxOwnedOpts struct {
	version string // mosaic_version stamp
	model   string // empty: no model key
	sandbox string // empty: read-only
	extra   string // extra top-level TOML lines (a carried user key), each ending in a newline
	body    string // empty: cxOwnedBody
}

// codexOwnedFile renders a Codex agent file: stamp comment, owned keys, extra keys and the
// body as developer_instructions.
func codexOwnedFile(o cxOwnedOpts) []byte {
	if o.version == "" {
		o.version = "1.0"
	}
	if o.sandbox == "" {
		o.sandbox = "read-only"
	}
	if o.body == "" {
		o.body = cxOwnedBody
	}
	var b strings.Builder
	b.WriteString("# mosaic_version: " + o.version + "\n\n")
	b.WriteString("name = \"runner\"\ndescription = \"Runs things\"\n")
	if o.model != "" {
		b.WriteString("model = \"" + o.model + "\"\n")
	}
	b.WriteString("sandbox_mode = \"" + o.sandbox + "\"\n")
	b.WriteString(o.extra)
	b.WriteString("developer_instructions = '''\n" + o.body + "'''\n")
	return []byte(b.String())
}

// probedOwned probes the file content at agent.<ext> under a fresh workspace and returns its
// record, failing the test when the probe filled none.
func probedOwned(t *testing.T, name string, content []byte, desc *domain.HarnessDescriptor) *domain.OwnedContent {
	t.Helper()
	ws := t.TempDir()
	writeFile(t, ws, name, content)
	state := probeDeployedArtifact(ws, name, "model", domain.ArtifactAgent, desc)
	if state.ParseFailed || state.Owned == nil {
		t.Fatalf("probe of %s: Owned = %v, ParseFailed = %v (%s); want a record", name, state.Owned, state.ParseFailed, state.ParseProblem)
	}
	return state.Owned
}

// canonicalOf decodes a Codex file to its canonical form through the registered translator.
func canonicalOf(t *testing.T, formatID string, raw []byte) []byte {
	t.Helper()
	tr, err := agentformat.LookupString(formatID)
	if err != nil {
		t.Fatalf("LookupString(%q): %v", formatID, err)
	}
	canonical, _, err := tr.Decode(raw, agentformat.ArtifactContext{Kind: domain.ArtifactAgent})
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return canonical
}

// codexProbed probes a Codex agent file.
func codexProbed(t *testing.T, o cxOwnedOpts) *domain.OwnedContent {
	t.Helper()
	return probedOwned(t, "agent.toml", codexOwnedFile(o), codexProbeDesc())
}

func TestProbeOwned_CodexAgent_RecordIsComputedFromDecodedForm(t *testing.T) {
	raw := codexOwnedFile(cxOwnedOpts{})

	got := probedOwned(t, "agent.toml", raw, codexProbeDesc())

	want, err := transform.OwnedContentOf(domain.ArtifactAgent, canonicalOf(t, "codex-toml", raw))
	if err != nil {
		t.Fatalf("OwnedContentOf(canonical): %v", err)
	}
	if got.Hash != want.Hash || len(got.Regions) != len(want.Regions) || len(want.Regions) < 2 {
		t.Errorf("Owned = %+v; want the record of the decoded form %+v (with region hashes)", got, want)
	}
}

func TestProbeOwned_CodexAgent_FrontmatterAndCarriedKeysDoNotAffectRecord(t *testing.T) {
	base := codexProbed(t, cxOwnedOpts{model: "gpt-5.6-sol"})
	cases := map[string]cxOwnedOpts{
		"model edited":        {model: "gpt-5.6-terra"},
		"model removed":       {},
		"sandbox_mode edited": {model: "gpt-5.6-sol", sandbox: "workspace-write"},
		"carried user key":    {model: "gpt-5.6-sol", extra: "user_key = \"keep-me\"\n"},
		"stamp edited":        {model: "gpt-5.6-sol", version: "9.9"},
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			got := codexProbed(t, opts)

			if got.Hash != base.Hash {
				t.Errorf("Owned.Hash = %s; want %s: frontmatter is not part of the record", got.Hash, base.Hash)
			}
		})
	}
}

func TestProbeOwned_CodexAgent_BodyEditOutsideRegionsNamesOutsideRegions(t *testing.T) {
	base := codexProbed(t, cxOwnedOpts{})

	got := codexProbed(t, cxOwnedOpts{body: strings.Replace(cxOwnedBody, "Outside prose.", "Outside prose, edited.", 1)})

	if got.Hash == base.Hash {
		t.Fatal("Owned.Hash unchanged by an edit of the body outside regions")
	}
	if diff := transform.DiffRegions(*base, *got); len(diff) != 1 || diff[0] != domain.OutsideRegionsName {
		t.Errorf("DiffRegions = %v; want [%s]", diff, domain.OutsideRegionsName)
	}
}

func TestProbeOwned_CodexAgent_DeployOwnedRegionEditNamesRegion(t *testing.T) {
	base := codexProbed(t, cxOwnedOpts{})

	got := codexProbed(t, cxOwnedOpts{body: strings.Replace(cxOwnedBody, "Identity prose.", "Identity prose, edited.", 1)})

	if got.Hash == base.Hash {
		t.Fatal("Owned.Hash unchanged by an edit of a Deploy-owned region")
	}
	if diff := transform.DiffRegions(*base, *got); len(diff) != 1 || diff[0] != "Identity" {
		t.Errorf("DiffRegions = %v; want [Identity]", diff)
	}
}

func TestProbeOwned_CodexAgent_ProjectRegionEditDoesNotAffectRecord(t *testing.T) {
	base := codexProbed(t, cxOwnedOpts{})

	got := codexProbed(t, cxOwnedOpts{body: strings.Replace(cxOwnedBody,
		"<IdentityExtension type=\"project\">\n", "<IdentityExtension type=\"project\">\nTeam rules.\n", 1)})

	if got.Hash != base.Hash {
		t.Errorf("Owned.Hash = %s; want %s: a project region is not part of the record", got.Hash, base.Hash)
	}
}

func TestProbeOwned_CodexAgent_CustomRegionEditDoesNotAffectRecord(t *testing.T) {
	base := codexProbed(t, cxOwnedOpts{})

	got := codexProbed(t, cxOwnedOpts{body: strings.Replace(cxOwnedBody,
		"<TeamNotes type=\"custom\">\n", "<TeamNotes type=\"custom\">\nMy notes.\n", 1)})

	if got.Hash != base.Hash {
		t.Errorf("Owned.Hash = %s; want %s: a custom region is not part of the record", got.Hash, base.Hash)
	}
}

func TestProbeOwned_MarkdownAgent_RecordIsUnchangedWholeFileBasis(t *testing.T) {
	md := "---\nid: 1\nversion: \"1.0\"\nname: runner\n---\n\n" + cxOwnedBody
	desc := &domain.HarnessDescriptor{Extensions: map[domain.ArtifactKind]string{domain.ArtifactAgent: ".md"}}

	got := probedOwned(t, "agent.md", []byte(md), desc)

	want, err := transform.OwnedContentOf(domain.ArtifactAgent, []byte(md))
	if err != nil {
		t.Fatalf("OwnedContentOf: %v", err)
	}
	if got.Hash != want.Hash || len(got.Regions) != len(want.Regions) {
		t.Errorf("Owned = %+v; want the unchanged Markdown record %+v", got, want)
	}
}

func TestProbeOwned_MarkdownAgent_FrontmatterEditDoesNotAffectRecord(t *testing.T) {
	desc := &domain.HarnessDescriptor{Extensions: map[domain.ArtifactKind]string{domain.ArtifactAgent: ".md"}}
	base := probedOwned(t, "a.md", []byte("---\nid: 1\nname: runner\n---\n\n"+cxOwnedBody), desc)

	got := probedOwned(t, "b.md", []byte("---\nid: 1\nname: runner\nmodel: other\nuser_key: x\n---\n\n"+cxOwnedBody), desc)

	if got.Hash != base.Hash {
		t.Errorf("Owned.Hash = %s; want %s: frontmatter is not part of the record", got.Hash, base.Hash)
	}
}

func TestProbeOwned_SkillRecordStaysWholeFile(t *testing.T) {
	skill := []byte("---\nmosaic_id: lean-tdd\nversion: \"1.0\"\n---\n\nSkill body.\n")
	ws := t.TempDir()
	writeFile(t, ws, "SKILL.md", skill)

	state := probeDeployedArtifact(ws, "SKILL.md", "model", domain.ArtifactSkill, &domain.HarnessDescriptor{})

	want, err := transform.OwnedContentOf(domain.ArtifactSkill, skill)
	if err != nil || state.Owned == nil || state.Owned.Hash != want.Hash {
		t.Errorf("Owned = %+v (err %v); want the whole-file record %+v", state.Owned, err, want)
	}
}
