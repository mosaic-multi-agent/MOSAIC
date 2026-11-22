package opencodev2_test

// Tests for the V2 spawn plan: one standalone `run` per subject invocation
// with the subject's model, the prompt on stdin, the sandbox subject
// directory as working directory, user config relocated through
// XDG_CONFIG_HOME, and a PWD entry that agrees with the working directory.
// Building the plan starts no process and writes nothing.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	commonharness "mosaic-common/harness"

	"mosaic-agent-test/internal/domain"
	"mosaic-agent-test/internal/harness/opencodev2"
)

func spawnTestSubject() domain.SubjectUnderTest {
	return domain.SubjectUnderTest{
		Identity:       "orchestrator",
		DefinitionPath: "agents/orchestrator.md",
		OpeningMessage: "begin the run",
		Model:          "anthropic/test-model",
	}
}

func spawnPlanFor(t *testing.T, subject domain.SubjectUnderTest) (domain.Sandbox, domain.SpawnPlan) {
	t.Helper()
	sb := newSandbox(t, t.TempDir())
	plan, err := opencodev2.New(opencodev2.Options{}).SpawnPlan(testContext(), subject, domain.Provisioning{Sandbox: sb})
	if err != nil {
		t.Fatalf("SpawnPlan: %v", err)
	}
	return sb, plan
}

func TestSpawnPlan_ArgsAreStandaloneRunWithModel(t *testing.T) {
	_, plan := spawnPlanFor(t, spawnTestSubject())

	want := []string{"run", "--agent", "orchestrator", "--format", "json", "--auto", "--standalone", "--model", "anthropic/test-model"}
	if !reflect.DeepEqual(plan.Args, want) {
		t.Errorf("Args = %v, want %v", plan.Args, want)
	}
}

func TestSpawnPlan_ArgsMatchSharedV2ArgumentBuilder(t *testing.T) {
	subject := spawnTestSubject()
	sb, plan := spawnPlanFor(t, subject)

	wantArgs, wantStdin, err := commonharness.BuildOpenCodeV2Args(commonharness.SpawnRequest{
		Agent:        commonharness.AgentRef{Identifier: subject.Identity},
		Prompt:       subject.OpeningMessage,
		SystemPrompt: commonharness.EnvBlock(sb.SubjectDir),
		Model:        subject.Model,
		OutputFormat: "json",
	})
	if err != nil {
		t.Fatalf("BuildOpenCodeV2Args (reference): %v", err)
	}
	if !reflect.DeepEqual(plan.Args, wantArgs) {
		t.Errorf("Args = %v, want the shared builder's %v", plan.Args, wantArgs)
	}
	if !bytes.Equal(plan.Stdin, wantStdin) {
		t.Errorf("Stdin = %q, want the shared builder's %q", plan.Stdin, wantStdin)
	}
}

func TestSpawnPlan_ModelWithVariantIsPassedVerbatim(t *testing.T) {
	subject := spawnTestSubject()
	subject.Model = "anthropic/test-model#high"

	_, plan := spawnPlanFor(t, subject)

	n := len(plan.Args)
	if n < 2 || plan.Args[n-2] != "--model" || plan.Args[n-1] != "anthropic/test-model#high" {
		t.Errorf("Args = %v, want it to end with --model anthropic/test-model#high", plan.Args)
	}
}

func TestSpawnPlan_StdinCarriesEnvBlockForSandboxAndOpeningMessage(t *testing.T) {
	subject := spawnTestSubject()
	sb, plan := spawnPlanFor(t, subject)

	want := commonharness.EnvBlock(sb.SubjectDir) + "\n" + subject.OpeningMessage
	if string(plan.Stdin) != want {
		t.Errorf("Stdin = %q, want %q", plan.Stdin, want)
	}
}

func TestSpawnPlan_ArgsCarryNoPromptNoDirNoSessionFlags(t *testing.T) {
	subject := spawnTestSubject()
	_, plan := spawnPlanFor(t, subject)

	for _, arg := range plan.Args {
		if strings.Contains(arg, subject.OpeningMessage) || strings.ContainsAny(arg, "\r\n") {
			t.Errorf("Args = %v, want no element carrying prompt content", plan.Args)
		}
		switch arg {
		case "--dir", "--variant", "--session", "--continue", "--fork":
			t.Errorf("Args = %v, want no %q", plan.Args, arg)
		}
	}
}

func TestSpawnPlan_ExecutableIsOpenCode(t *testing.T) {
	_, plan := spawnPlanFor(t, spawnTestSubject())

	if plan.Executable != "opencode" {
		t.Errorf("Executable = %q, want %q", plan.Executable, "opencode")
	}
}

func TestSpawnPlan_WorkingDirIsSandboxSubjectDir(t *testing.T) {
	sb, plan := spawnPlanFor(t, spawnTestSubject())

	if plan.WorkingDir != sb.SubjectDir {
		t.Errorf("WorkingDir = %q, want %q", plan.WorkingDir, sb.SubjectDir)
	}
}

func envValue(env []string, key string) (string, bool) {
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, key+"="); ok {
			return v, true
		}
	}
	return "", false
}

// TestSpawnPlan_EnvCarriesRelocatedConfigHomeAndPWD pins the plan's
// environment entries by key: the relocated config home and PWD equal to the
// subject directory.
func TestSpawnPlan_EnvCarriesRelocatedConfigHomeAndPWD(t *testing.T) {
	sb, plan := spawnPlanFor(t, spawnTestSubject())

	subjectAbs, err := filepath.Abs(sb.SubjectDir)
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if got, ok := envValue(plan.Env, "XDG_CONFIG_HOME"); !ok || got != filepath.Join(sb.ControlDir, "opencode-v2-home") {
		t.Errorf("Env XDG_CONFIG_HOME = %q (present %v), want %q", got, ok, filepath.Join(sb.ControlDir, "opencode-v2-home"))
	}
	if got, ok := envValue(plan.Env, "PWD"); !ok || got != subjectAbs {
		t.Errorf("Env PWD = %q (present %v), want %q", got, ok, subjectAbs)
	}
}

func TestSpawnPlan_RunsAgentInPlaceWithoutRewritingDefinition(t *testing.T) {
	subject := spawnTestSubject()
	sb := newSandbox(t, t.TempDir())
	def := filepath.Join(sb.SubjectDir, filepath.FromSlash(subject.DefinitionPath))
	writeProjectFile(t, def, "original definition")
	before := listDir(t, sb.SubjectDir)

	plan, err := opencodev2.New(opencodev2.Options{}).SpawnPlan(testContext(), subject, domain.Provisioning{Sandbox: sb})
	if err != nil {
		t.Fatalf("SpawnPlan: %v", err)
	}

	if data, rerr := os.ReadFile(def); rerr != nil || string(data) != "original definition" {
		t.Errorf("definition file changed or vanished: %q, %v", data, rerr)
	}
	if after := listDir(t, sb.SubjectDir); !reflect.DeepEqual(before, after) {
		t.Errorf("SpawnPlan changed the subject dir from %v to %v", before, after)
	}
	if len(plan.Args) < 3 || plan.Args[1] != "--agent" || plan.Args[2] != subject.Identity {
		t.Errorf("Args = %v, want the agent selected by id", plan.Args)
	}
}

func TestSpawnPlan_SetsNoMosaicVariables(t *testing.T) {
	_, plan := spawnPlanFor(t, spawnTestSubject())

	if len(plan.Env) == 0 {
		t.Fatalf("Env is empty, want the config home and PWD entries")
	}
	for _, e := range plan.Env {
		if strings.HasPrefix(e, "MOSAIC_") {
			t.Errorf("Env carries %q, want no MOSAIC_ variable", e)
		}
	}
}

func TestSpawnPlan_TimeoutIsBackstopAndSentinelMatchesSandbox(t *testing.T) {
	sb, plan := spawnPlanFor(t, spawnTestSubject())

	if plan.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want 30m", plan.Timeout)
	}
	if plan.EarlyExitSentinel != sb.EarlyExitSentinelPath() {
		t.Errorf("EarlyExitSentinel = %q, want %q", plan.EarlyExitSentinel, sb.EarlyExitSentinelPath())
	}
	if plan.DiagnosticLog != "" || plan.RunID != "" {
		t.Errorf("DiagnosticLog = %q, RunID = %q, want both left empty for the runner to stamp", plan.DiagnosticLog, plan.RunID)
	}
}

func TestSpawnPlan_WithoutModelIsRefused(t *testing.T) {
	subject := spawnTestSubject()
	subject.Model = ""
	a := opencodev2.New(opencodev2.Options{})

	plan, err := a.SpawnPlan(testContext(), subject, domain.Provisioning{Sandbox: newSandbox(t, t.TempDir())})

	if !errors.Is(err, domain.ErrNoSubjectModel) {
		t.Fatalf("SpawnPlan error = %v, want one wrapping domain.ErrNoSubjectModel", err)
	}
	if !strings.HasPrefix(err.Error(), "opencode-v2: ") {
		t.Errorf("SpawnPlan error = %q, want the %q prefix", err.Error(), "opencode-v2: ")
	}
	if !reflect.DeepEqual(plan, domain.SpawnPlan{}) {
		t.Errorf("SpawnPlan = %+v, want the zero plan", plan)
	}
}

func TestSpawnPlan_EmptyIdentityIsRefusedAsBuilderError(t *testing.T) {
	subject := spawnTestSubject()
	subject.Identity = ""
	a := opencodev2.New(opencodev2.Options{})

	plan, err := a.SpawnPlan(testContext(), subject, domain.Provisioning{Sandbox: newSandbox(t, t.TempDir())})

	if !errors.Is(err, commonharness.ErrOpenCodeV2EmptyAgentIdentifier) {
		t.Fatalf("SpawnPlan error = %v, want one wrapping ErrOpenCodeV2EmptyAgentIdentifier", err)
	}
	if !strings.Contains(err.Error(), "building spawn arguments") {
		t.Errorf("SpawnPlan error = %q, want it to say building spawn arguments failed", err.Error())
	}
	if !reflect.DeepEqual(plan, domain.SpawnPlan{}) {
		t.Errorf("SpawnPlan = %+v, want the zero plan", plan)
	}
}

func TestSpawnPlan_BuildingItTouchesNoFilesAndRepeatsEquivalently(t *testing.T) {
	a := opencodev2.New(opencodev2.Options{})
	sb := newSandbox(t, t.TempDir())
	prov := domain.Provisioning{Sandbox: sb}
	before := listDir(t, sb.Root)

	first, err := a.SpawnPlan(testContext(), spawnTestSubject(), prov)
	if err != nil {
		t.Fatalf("SpawnPlan (first): %v", err)
	}
	second, err := a.SpawnPlan(testContext(), spawnTestSubject(), prov)
	if err != nil {
		t.Fatalf("SpawnPlan (second): %v", err)
	}

	if after := listDir(t, sb.Root); !reflect.DeepEqual(before, after) {
		t.Errorf("sandbox changed from %v to %v; SpawnPlan must only describe", before, after)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("first plan = %+v, second plan = %+v, want equivalent plans", first, second)
	}
}

func listDir(t *testing.T, root string) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(root, func(p string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		names = append(names, p)
		return nil
	})
	if err != nil {
		t.Fatalf("listDir(%q): %v", root, err)
	}
	return names
}
