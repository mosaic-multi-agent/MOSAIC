package opencodev2_test

// Executes the V2 spawn plan through the real launcher against a fake child
// process (this test binary in helper mode) and checks the environment the
// child actually receives. A stale PWD inherited from the parent is exactly
// the condition that makes OpenCode V2 resolve the wrong project, so the plan
// is judged by the child's effective environment, not only by its fields.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencodev2"
	"mosaic-agent-test/internal/launch"
)

// childObservation is what the helper process reports about itself.
type childObservation struct {
	Cwd       string `json:"cwd"`
	PWD       string `json:"pwd"`
	PWDSet    bool   `json:"pwd_set"`
	XDGConfig string `json:"xdg_config_home"`
}

// runEnvironmentHelper is the fake child: it records its own working
// directory and the PWD / XDG_CONFIG_HOME it received, then exits.
func runEnvironmentHelper() {
	obs := childObservation{}
	obs.Cwd, _ = os.Getwd()
	obs.PWD, obs.PWDSet = os.LookupEnv("PWD")
	obs.XDGConfig = os.Getenv("XDG_CONFIG_HOME")
	data, _ := json.Marshal(obs)
	if out := os.Getenv("GO_HELPER_OUT"); out != "" {
		os.WriteFile(out, data, 0o644) //nolint:errcheck
	}
	os.Exit(0)
}

// launchPlanAgainstHelper runs plan through the launcher with the helper
// binary as the executable and returns what the child observed.
func launchPlanAgainstHelper(t *testing.T, plan domain.SpawnPlan) childObservation {
	t.Helper()
	out := filepath.Join(t.TempDir(), "child.json")
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("GO_HELPER_OUT", out)

	plan.Executable = os.Args[0]
	if _, err := launch.New(opencodev2.DecodeEnvelope).Launch(testContext(), plan); err != nil {
		t.Fatalf("Launch: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the child never reported its environment: %v", err)
	}
	var obs childObservation
	if err := json.Unmarshal(data, &obs); err != nil {
		t.Fatalf("decoding child report %q: %v", data, err)
	}
	return obs
}

func TestSpawnPlanThroughLauncher_ChildPWDAgreesDespiteStaleInheritedPWD(t *testing.T) {
	sb, plan := spawnPlanFor(t, spawnTestSubject())
	stale := t.TempDir()
	t.Setenv("PWD", stale)

	obs := launchPlanAgainstHelper(t, plan)

	subjectInfo, err := os.Stat(sb.SubjectDir)
	if err != nil {
		t.Fatalf("stat subject dir: %v", err)
	}
	cwdInfo, err := os.Stat(obs.Cwd)
	if err != nil {
		t.Fatalf("stat child cwd %q: %v", obs.Cwd, err)
	}
	if !os.SameFile(subjectInfo, cwdInfo) {
		t.Fatalf("child cwd = %q, want the sandbox subject dir %q", obs.Cwd, sb.SubjectDir)
	}
	if obs.PWDSet && filepath.Clean(obs.PWD) != filepath.Clean(sb.SubjectDir) {
		t.Errorf("child PWD = %q, want it to equal the subject dir %q (stale parent PWD was %q) or be absent", obs.PWD, sb.SubjectDir, stale)
	}
}

func TestSpawnPlanThroughLauncher_ChildReceivesRelocatedConfigHome(t *testing.T) {
	sb, plan := spawnPlanFor(t, spawnTestSubject())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	obs := launchPlanAgainstHelper(t, plan)

	want := filepath.Join(sb.ControlDir, "opencode-v2-home")
	if obs.XDGConfig != want {
		t.Errorf("child XDG_CONFIG_HOME = %q, want %q", obs.XDGConfig, want)
	}
}
