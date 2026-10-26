package app

// stamp_recognition_test.go covers recognition of MOSAIC-stamped agent files by the shared
// eligibility gate: files written by the current deploy (new harness version stamp), files
// carrying only the legacy transform version stamp, and the reason given for unstamped files.
// Current-deploy inputs are the tool-local golden outputs; only recognition is asserted.

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mosaic-deploy/internal/agentfields"
	"mosaic-deploy/internal/domain"
)

// goldenCurrentDeployAgents returns name -> bytes of the non-orchestrator golden agent
// files, one per golden harness directory.
func goldenCurrentDeployAgents(t *testing.T) map[string][]byte {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "golden", "*", "planner-tdd-soft*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no golden agent files found: %v", err)
	}
	out := map[string][]byte{}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		out[filepath.Base(filepath.Dir(p))] = data
	}
	return out
}

// harnessStampLine matches the whole current-deploy stamp line of a golden file.
var harnessStampLine = regexp.MustCompile(`(?m)^mosaic_harness_version:[ \t]*(.*)\n`)

// goldenStampValue returns the stamp value the golden current-deploy file carries.
func goldenStampValue(t *testing.T, src []byte) string {
	t.Helper()
	m := harnessStampLine.FindSubmatch(src)
	if m == nil {
		t.Fatal("golden file carries no mosaic_harness_version line")
	}
	return strings.Trim(strings.TrimSpace(string(m[1])), `"'`)
}

// withoutStamp removes the current-deploy stamp line from the golden file.
func withoutStamp(src []byte) []byte {
	return harnessStampLine.ReplaceAll(src, nil)
}

// withStampLine replaces the current-deploy stamp line with the given raw YAML text.
func withStampLine(src []byte, line string) []byte {
	return harnessStampLine.ReplaceAll(src, []byte(line+"\n"))
}

// withStampKey rewrites the golden current-deploy stamp key to another key name.
func withStampKey(src []byte, key string) []byte {
	return []byte(strings.Replace(string(src), "mosaic_harness_version:", key+":", 1))
}

func TestCurrentDeployOutput_IsNotSkippedAsNotAgentByHarnessMatch(t *testing.T) {
	for harness, src := range goldenCurrentDeployAgents(t) {
		t.Run(harness, func(t *testing.T) {
			verdict := DetectHarnessMatch(src, newAlphaHarnessModule(), domain.ArtifactAgent, "planner-tdd-soft")

			switch verdict.Status {
			case HarnessMatchYes, HarnessMatchNo, HarnessMatchIndeterminate:
			default:
				t.Errorf("current-deploy file status = %q, want a harness match classification: %s",
					verdict.Status, verdict.Reason)
			}
		})
	}
}

func TestCurrentDeployOutput_IsAcceptedByPromote(t *testing.T) {
	for harness, src := range goldenCurrentDeployAgents(t) {
		t.Run(harness, func(t *testing.T) {
			_, err := buildGenericAgent(PromoteInput{Source: src, NumericID: "99", Key: "planner-tdd-soft"})

			if errors.Is(err, ErrPromoteNotTransformed) {
				t.Errorf("current-deploy file refused as not transformed: %v", err)
			} else if err != nil {
				t.Errorf("current-deploy file refused: %v", err)
			}
		})
	}
}

func TestCurrentDeployOutput_IsReturnedByHarnessOnlyDiscovery(t *testing.T) {
	for harness, src := range goldenCurrentDeployAgents(t) {
		t.Run(harness, func(t *testing.T) {
			ws := t.TempDir()
			dir := filepath.Join(ws, "agents")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "my-agent.md"), src, 0o644); err != nil {
				t.Fatal(err)
			}

			got := scanHarnessOnlyAgents(ws, "agents", map[string]bool{})

			if len(got) != 1 {
				t.Fatalf("scanHarnessOnlyAgents returned %d agents, want 1", len(got))
			}
			if got[0].TransformVersion != goldenStampValue(t, src) {
				t.Errorf("discovered agent stamp value = %q, want %q", got[0].TransformVersion, goldenStampValue(t, src))
			}
		})
	}
}

func TestEligibleHarnessOnly_EachStampKeyAlone_IsRecognised(t *testing.T) {
	golden := goldenCurrentDeployAgents(t)["opencode"]
	value := goldenStampValue(t, golden)
	for _, key := range []string{
		"mosaic_harness_version", "harness_version",
		"mosaic_transform_version", "transform_version",
	} {
		t.Run(key, func(t *testing.T) {
			verdict := eligibleHarnessOnly(withStampKey(golden, key))

			if !verdict.Eligible {
				t.Errorf("file stamped only with %q not recognised: %s", key, verdict.Reason)
			}
			if verdict.Meta.TransformVersion != value {
				t.Errorf("stamp value for %q = %q, want %q", key, verdict.Meta.TransformVersion, value)
			}
		})
	}
}

func TestEligibleHarnessOnly_OldAndNewStampsTogether_NewStampValueWins(t *testing.T) {
	golden := goldenCurrentDeployAgents(t)["opencode"]
	newValue := goldenStampValue(t, golden)
	src := withStampLine(golden, "mosaic_transform_version: 0.0.1-legacy\nmosaic_harness_version: "+newValue)

	verdict := eligibleHarnessOnly(src)

	if !verdict.Eligible {
		t.Fatalf("file carrying both stamps not recognised: %s", verdict.Reason)
	}
	if verdict.Meta.TransformVersion != newValue {
		t.Errorf("stamp value = %q, want the new stamp %q", verdict.Meta.TransformVersion, newValue)
	}
}

func TestEligibleHarnessOnly_PrefixedAndLegacyOfOneStamp_PrefixedValueWins(t *testing.T) {
	golden := goldenCurrentDeployAgents(t)["opencode"]
	newValue := goldenStampValue(t, golden)
	src := withStampLine(golden, "harness_version: 0.0.1-legacy\nmosaic_harness_version: "+newValue)

	verdict := eligibleHarnessOnly(src)

	if !verdict.Eligible {
		t.Fatalf("file carrying prefixed and legacy stamp not recognised: %s", verdict.Reason)
	}
	if verdict.Meta.TransformVersion != newValue {
		t.Errorf("stamp value = %q, want the prefixed value %q", verdict.Meta.TransformVersion, newValue)
	}
}

func TestEligibleHarnessOnly_EmptyValuedStamp_IsNotRecognised(t *testing.T) {
	golden := goldenCurrentDeployAgents(t)["opencode"]
	src := withStampLine(golden, `mosaic_harness_version: ""`)

	verdict := eligibleHarnessOnly(src)

	if verdict.Eligible {
		t.Error("file with only an empty-valued stamp was recognised")
	}
}

func TestEligibleHarnessOnly_NonScalarStamp_IsNotRecognised(t *testing.T) {
	golden := goldenCurrentDeployAgents(t)["opencode"]
	for name, line := range map[string]string{
		"list": "mosaic_harness_version: [a]",
		"map":  "mosaic_harness_version:\n  a: b",
	} {
		t.Run(name, func(t *testing.T) {
			verdict := eligibleHarnessOnly(withStampLine(golden, line))

			if verdict.Eligible {
				t.Error("file with only a non-scalar stamp was recognised")
			}
		})
	}
}

func TestEligibleHarnessOnly_UnstampedFile_ReasonNamesFieldsChecked(t *testing.T) {
	golden := goldenCurrentDeployAgents(t)["opencode"]
	keys := agentfields.VersionStampKeys()
	if len(keys) == 0 {
		t.Fatal("agentfields.VersionStampKeys() is empty")
	}

	verdict := eligibleHarnessOnly(withoutStamp(golden))

	if verdict.Eligible {
		t.Fatal("unstamped file was recognised")
	}
	// Both stamp families must be named, plus every key the shared decision checks.
	wanted := append([]string{"harness_version", "transform_version"}, keys...)
	for _, key := range wanted {
		if !strings.Contains(verdict.Reason, key) {
			t.Errorf("reason %q does not name checked field %q", verdict.Reason, key)
		}
	}
}

func TestPromote_UnstampedFile_ErrorReasonNamesHarnessVersionField(t *testing.T) {
	golden := goldenCurrentDeployAgents(t)["opencode"]

	_, err := buildGenericAgent(PromoteInput{Source: withoutStamp(golden), NumericID: "99", Key: "x"})

	if !errors.Is(err, ErrPromoteNotTransformed) {
		t.Fatalf("err = %v, want ErrPromoteNotTransformed", err)
	}
	if !strings.Contains(err.Error(), "harness_version") {
		t.Errorf("promote refusal %q does not name the harness version stamp field", err.Error())
	}
}
