package app_test

// thirdformat_parity_observe_test.go builds the observation of one twin side after a flow step:
// the plan items, the executed actions, the decoded deployed agents of the twin and of every
// counterpart harness, and the normalised manifest. Agents are observed in decoded (canonical)
// form so that the byte encodings of the two formats never enter the comparison.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	yaml "github.com/goccy/go-yaml"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/domain"
)

// twinObservation is what a flow step produces, compared field by field under the parity rule.
type twinObservation struct {
	Items   []twinPlanItem       // plan items of the run, in plan order
	Actions []twinAction         // executed actions, in summary order
	Agents  map[string]twinAgent // agent key -> decoded deployed twin agent after the step
	// Counterparts: "<counterpart harness id>/<agent key>" -> decoded deployed agent of each
	// counterpart harness in the side's workspace after the step; nil without counterparts.
	Counterparts        map[string]twinAgent
	Manifest            []domain.ManifestEntry // normalised: no ContentHash, no DeployedAt, "<ext>" in twin agent paths
	OwnedKeyDifferences []domain.OwnedKeyDifference
	Flow                *twinFlowResult // set by runTwinFlowStep only

	workspace string
	ext       string // the twin's agent file extension
}

type twinPlanItem struct {
	Ref               domain.ArtifactRef
	TargetPath        string // agent extension replaced by "<ext>"
	Action            domain.PlanAction
	Reasons           []string
	BackupBeforeWrite bool
}

type twinAction struct {
	Ref      domain.ArtifactRef
	Taken    domain.ActionTaken
	BackedUp bool // BackupPath != ""
}

type twinAgent struct {
	Frontmatter map[string]string // key -> rendered canonical value, carriage keys excluded
	Body        string            // canonical body
}

// twinExtOf is the agent file extension of a twin format.
func twinExtOf(f twinFormat) string {
	if f == twinTestFormat {
		return ".json"
	}
	return ".md"
}

// twinExtPlaceholder replaces the twin agent extension in compared paths.
const twinExtPlaceholder = "<ext>"

// twinNormalPath replaces the twin agent extension of an agent path.
func twinNormalPath(path, ext string) string {
	if strings.HasSuffix(path, ext) {
		return strings.TrimSuffix(path, ext) + twinExtPlaceholder
	}
	return path
}

func twinPlanItems(side twinSide, p domain.Plan) []twinPlanItem {
	ext := twinExtOf(side.Format)
	var out []twinPlanItem
	for _, it := range p.Items {
		path := it.TargetPath
		if it.Ref.Kind == domain.ArtifactAgent {
			path = twinNormalPath(filepath.ToSlash(path), ext)
		}
		out = append(out, twinPlanItem{Ref: it.Ref, TargetPath: path, Action: it.Action,
			Reasons: it.Reasons, BackupBeforeWrite: it.BackupBeforeWrite})
	}
	return out
}

func twinActions(sum domain.RunSummary) []twinAction {
	var out []twinAction
	for _, a := range sum.Actions {
		out = append(out, twinAction{Ref: a.Ref, Taken: a.Taken, BackedUp: a.BackupPath != ""})
	}
	return out
}

// observeTwinSide reads the workspace of a side: decoded agents and the normalised manifest.
func observeTwinSide(t *testing.T, side twinSide) twinObservation {
	t.Helper()
	obs := twinObservation{workspace: side.Workspace, ext: twinExtOf(side.Format),
		Agents: twinDeployedAgents(t, side.Workspace, side.Module)}
	for _, cp := range side.Counterparts {
		if obs.Counterparts == nil {
			obs.Counterparts = map[string]twinAgent{}
		}
		for key, a := range twinDeployedAgents(t, side.Workspace, cp) {
			obs.Counterparts[cp.Ref().ID+"/"+key] = a
		}
	}
	obs.Manifest = twinNormalManifest(t, side)
	return obs
}

// twinDeployedAgents decodes every agent file in the agents folder of mod. A file its
// translator cannot decode, or whose canonical form has no parseable frontmatter, is left out.
func twinDeployedAgents(t *testing.T, ws string, mod domain.HarnessModule) map[string]twinAgent {
	t.Helper()
	desc := mod.Descriptor()
	ext := desc.Extensions[domain.ArtifactAgent]
	dir := filepath.Join(ws, filepath.FromSlash(desc.Paths.Agents.Project))
	files, err := os.ReadDir(dir)
	out := map[string]twinAgent{}
	if err != nil {
		return out
	}
	tr, err := agentformat.LookupString(desc.AgentFormatID)
	if err != nil {
		t.Fatalf("agent format %q of %s does not resolve: %v", desc.AgentFormatID, desc.ID, err)
	}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ext) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", f.Name(), err)
		}
		canonical, _, err := tr.Decode(raw, agentformat.ArtifactContext{Kind: domain.ArtifactAgent})
		if err != nil {
			continue
		}
		if a, ok := parseTwinCanonical(canonical); ok {
			out[strings.TrimSuffix(f.Name(), ext)] = a
		}
	}
	return out
}

// parseTwinCanonical splits canonical agent bytes into frontmatter values and body.
func parseTwinCanonical(canonical []byte) (twinAgent, bool) {
	s := strings.ReplaceAll(string(canonical), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return twinAgent{}, false
	}
	rest := s[3:]
	end := strings.Index(rest, "\n---\n")
	tail := 5
	if end < 0 && strings.HasSuffix(rest, "\n---") {
		end, tail = len(rest)-4, 4
	}
	if end < 0 {
		return twinAgent{}, false
	}
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(rest[:end]), &raw); err != nil {
		return twinAgent{}, false
	}
	fm := map[string]string{}
	for k, v := range raw {
		if k == agentformat.CarriageValuesKey || k == agentformat.CarriageMarkersKey {
			continue
		}
		fm[k] = fmt.Sprint(v)
	}
	return twinAgent{Frontmatter: fm, Body: rest[end+tail:]}, true
}

// twinNormalManifest loads the manifest of the side and applies the parity normalisation.
func twinNormalManifest(t *testing.T, side twinSide) []domain.ManifestEntry {
	t.Helper()
	ext := twinExtOf(side.Format)
	var out []domain.ManifestEntry
	for _, e := range gamManifest(t, side.Workspace).Entries {
		e.ContentHash = ""
		e.DeployedAt = time.Time{}
		if e.Harness == twinHarnessID && e.Ref.Kind == domain.ArtifactAgent {
			e.TargetPath = twinNormalPath(filepath.ToSlash(e.TargetPath), ext)
		}
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		ka := fmt.Sprint(a.Harness, "|", a.Ref.Kind, "|", a.Ref.Key, "|", a.TargetPath)
		kb := fmt.Sprint(b.Harness, "|", b.Ref.Kind, "|", b.Ref.Key, "|", b.TargetPath)
		return ka < kb
	})
	return out
}
