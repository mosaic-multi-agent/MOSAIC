package harness_test

// Tests for BuildCodexArgs: the pure argv half of a `codex exec` invocation
// built from a loaded Codex agent. The agent applies through argv because Codex
// has no agent flag; instructions travel on stdin (see
// spawn_codex_stdin_test.go).

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"mosaic-common/harness"
)

func TestBuildCodexArgs_FixedShape_NonInteractiveJSONWithoutGitCheck(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{Prompt: "go"}, loadCodexAgent(t, "agent_full.toml"))

	if len(args) == 0 || args[0] != "exec" {
		t.Errorf("want argv to start with exec, got %v", args)
	}
	for _, flag := range []string{"--json", "--skip-git-repo-check"} {
		if !containsArg(args, flag) {
			t.Errorf("want %s in argv, got %v", flag, args)
		}
	}
	if args[len(args)-1] != "-" {
		t.Errorf("want argv to end with - (prompt on stdin), got %v", args)
	}
}

func TestBuildCodexArgs_WorkingDirectory_PassedWithC(t *testing.T) {
	agent := loadCodexAgent(t, "agent_full.toml")

	with, _ := buildCodex(t, harness.SpawnRequest{WorkingDir: "/work/space"}, agent)
	without, _ := buildCodex(t, harness.SpawnRequest{}, agent)

	if !containsSequence(with, "-C", "/work/space") {
		t.Errorf("want -C /work/space, got %v", with)
	}
	if containsArg(without, "-C") {
		t.Errorf("want no -C without a working directory, got %v", without)
	}
}

func TestBuildCodexArgs_ModelAndSandbox_FromAgentFile(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{}, loadCodexAgent(t, "agent_full.toml"))

	if !containsSequence(args, "-m", "gpt-5") {
		t.Errorf("want -m gpt-5 from the agent file, got %v", args)
	}
	if !containsSequence(args, "-s", "workspace-write") {
		t.Errorf("want -s workspace-write from the agent file, got %v", args)
	}
}

func TestBuildCodexArgs_RequestModel_OverridesAgentModel(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{Model: "gpt-5-codex"}, loadCodexAgent(t, "agent_full.toml"))

	if !containsSequence(args, "-m", "gpt-5-codex") {
		t.Errorf("want -m gpt-5-codex, got %v", args)
	}
	if containsSequence(args, "-m", "gpt-5") {
		t.Errorf("agent model must not also be passed, got %v", args)
	}
}

func TestBuildCodexArgs_AgentWithoutModelOrSandbox_OmitsBothFlags(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{}, loadCodexAgent(t, "agent_minimal.toml"))

	if containsArg(args, "-m") || containsArg(args, "-s") {
		t.Errorf("want no -m / -s when neither agent nor request declares them, got %v", args)
	}
}

func TestBuildCodexArgs_Unattended_ApprovalsDisabledAndContactUserApproved(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{}, loadCodexAgent(t, "agent_minimal.toml"))

	requireOverride(t, args, "approval_policy", `approval_policy="never"`)
	requireOverride(t, args, "mcp_servers.contact-user.default_tools_approval_mode",
		`mcp_servers.contact-user.default_tools_approval_mode="approve"`)
	for _, tool := range harness.CodexContactUserTools() {
		path := "mcp_servers." + harness.CodexContactUserServer + ".tools." + tool + ".approval_mode"
		requireOverride(t, args, path, path+`="approve"`)
	}
}

func TestCodexContactUserTools_ListsTheTwoToolsInOrder(t *testing.T) {
	got := harness.CodexContactUserTools()

	want := []string{"ask_user_questions", "get_answered_questions"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CodexContactUserTools() = %v, want %v", got, want)
	}
	got[0] = "changed"
	if again := harness.CodexContactUserTools(); again[0] != "ask_user_questions" {
		t.Errorf("each call must return a fresh slice, got %v", again)
	}
}

func TestBuildCodexArgs_DeclaredSettings_EmittedAsOverrides(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{}, loadCodexAgent(t, "agent_full.toml"))

	requireOverride(t, args, "web_search", `web_search="live"`)
	requireOverride(t, args, "pi", "pi=3.14")
	requireOverride(t, args, "retries", "retries=3")
	requireOverride(t, args, "verbose", "verbose=true")
	requireOverride(t, args, "since", "since=2026-01-02")
	requireOverride(t, args, "tags", `tags=["alpha", "beta"]`)
	requireOverride(t, args, "notes", `notes="first\nsecond"`)
}

func TestBuildCodexArgs_TableSettings_FlattenedToDottedLeaves(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{}, loadCodexAgent(t, "agent_full.toml"))

	requireOverride(t, args, "sandbox_workspace_write.network_access", "sandbox_workspace_write.network_access=true")
	requireOverride(t, args, "mcp_servers.docs.command", `mcp_servers.docs.command="docs-server"`)
}

func TestBuildCodexArgs_TableKeyThatIsNotBare_IsQuotedInPath(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{}, loadCodexAgent(t, "agent_full.toml"))

	requireOverride(t, args, `mcp_servers."my server".command`, `mcp_servers."my server".command="spaced-server"`)
}

func TestBuildCodexArgs_ArrayOfTables_IsOneInlineValue(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{}, loadCodexAgent(t, "agent_full.toml"))

	got := overridesFor(args, "extra_servers")

	if len(got) != 1 {
		t.Fatalf("want one override for the array of tables, got %v", got)
	}
	if !strings.HasPrefix(got[0], "extra_servers=[{") || !strings.HasSuffix(got[0], "}]") {
		t.Errorf("want an inline array of inline tables, got %q", got[0])
	}
	for _, part := range []string{`"tool-a"`, `"-y"`} {
		if !strings.Contains(got[0], part) {
			t.Errorf("array of tables override %q lacks %q", got[0], part)
		}
	}
	for _, kv := range configOverrides(args) {
		if strings.HasPrefix(kv, "extra_servers.") {
			t.Errorf("an array must not be flattened into leaves, got %q", kv)
		}
	}
}

func TestBuildCodexArgs_SettingsInFileOrder(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{}, loadCodexAgent(t, "agent_full.toml"))

	order := []string{"web_search=", "pi=", "retries=", "verbose=", "since="}
	last := -1
	overrides := configOverrides(args)
	for _, prefix := range order {
		idx := -1
		for i, kv := range overrides {
			if strings.HasPrefix(kv, prefix) {
				idx = i
			}
		}
		if idx <= last {
			t.Fatalf("override %q out of file order in %v", prefix, overrides)
		}
		last = idx
	}
}

func TestBuildCodexArgs_OwnedInstructionKeys_NotEmittedAsOverrides(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{}, loadCodexAgent(t, "agent_full.toml"))

	for _, key := range []string{"name", "description", "model", "sandbox_mode", "developer_instructions"} {
		if got := overridesFor(args, key); len(got) != 0 {
			t.Errorf("%s travels through its own channel, not -c: got %v", key, got)
		}
	}
}

func TestBuildCodexArgs_SettingCollidingWithFixedOverride_NotEmitted(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{}, loadCodexAgent(t, "agent_collision.toml"))

	requireOverride(t, args, "approval_policy", `approval_policy="never"`)
	requireOverride(t, args, "mcp_servers.contact-user.default_tools_approval_mode",
		`mcp_servers.contact-user.default_tools_approval_mode="approve"`)
	for _, tool := range harness.CodexContactUserTools() {
		path := "mcp_servers.contact-user.tools." + tool + ".approval_mode"
		requireOverride(t, args, path, path+`="approve"`)
	}
}

func TestBuildCodexArgs_OtherContactUserSetting_StillEmitted(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{}, loadCodexAgent(t, "agent_collision.toml"))

	requireOverride(t, args, "mcp_servers.contact-user.command", `mcp_servers.contact-user.command="contact-server"`)
}

func TestBuildCodexArgs_ExtraArgs_PrecedeTrailingDash(t *testing.T) {
	args, _ := buildCodex(t, harness.SpawnRequest{ExtraArgs: []string{"--extra-flag", "v"}}, loadCodexAgent(t, "agent_minimal.toml"))

	n := len(args)
	if n < 3 || args[n-3] != "--extra-flag" || args[n-2] != "v" || args[n-1] != "-" {
		t.Errorf("want ExtraArgs directly before the trailing -, got %v", args)
	}
}

func TestBuildCodexArgs_IgnoredRequestFields_DoNotChangeArgvOrStdin(t *testing.T) {
	agent := loadCodexAgent(t, "agent_full.toml")
	base := harness.SpawnRequest{Prompt: "p"}
	noisy := base
	noisy.Agent = harness.AgentRef{Identifier: "other-agent", DefinitionPath: "/nowhere.toml"}
	noisy.MaxTurns = 7
	noisy.AllowedTools = []string{"Read"}
	noisy.DerivedTools = []string{"Bash"}
	noisy.ToolsDerived = true
	noisy.SessionPersistence = true
	noisy.GHCPCLIMode = harness.GHCPCLIModeBlanket

	wantArgs, wantStdin := buildCodex(t, base, agent)
	gotArgs, gotStdin := buildCodex(t, noisy, agent)

	if !reflect.DeepEqual(gotArgs, wantArgs) || string(gotStdin) != string(wantStdin) {
		t.Errorf("ignored fields changed the invocation:\n got %v / %q\nwant %v / %q", gotArgs, gotStdin, wantArgs, wantStdin)
	}
}

func TestBuildCodexArgs_NilAgent_Rejected(t *testing.T) {
	args, stdin, err := harness.BuildCodexArgs(harness.SpawnRequest{Prompt: "p"}, nil)

	if !errors.Is(err, harness.ErrCodexNilAgent) {
		t.Fatalf("want ErrCodexNilAgent, got %v", err)
	}
	if args != nil || stdin != nil {
		t.Errorf("want nil args and stdin on error, got %v / %q", args, stdin)
	}
}

func TestBuildCodexArgs_OutputFormat_OnlyJSONOrEmptyAccepted(t *testing.T) {
	agent := loadCodexAgent(t, "agent_minimal.toml")

	for _, format := range []string{"", "json"} {
		if _, _, err := harness.BuildCodexArgs(harness.SpawnRequest{OutputFormat: format}, agent); err != nil {
			t.Errorf("OutputFormat %q: unexpected error %v", format, err)
		}
	}
	_, _, err := harness.BuildCodexArgs(harness.SpawnRequest{OutputFormat: "stream-json"}, agent)
	if !errors.Is(err, harness.ErrCodexUnsupportedOutputFormat) {
		t.Errorf("want ErrCodexUnsupportedOutputFormat for stream-json, got %v", err)
	}
}
