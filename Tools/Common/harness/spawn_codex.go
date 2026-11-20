// Codex invocation: argument building and the Spawner. Codex has no agent flag,
// so the caller applies the agent: its model, sandbox and every other declared
// setting travel as command-line overrides, its instructions travel on stdin
// with the prompt. The agent file is only read, never modified.
package harness

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mosaic-common/agentformat/formatid"
	"mosaic-common/agentload"
)

// CodexContactUserServer is the MCP server name MOSAIC deployments use for
// HITL; the invocation pre-approves its tools.
const CodexContactUserServer = "contact-user"

var (
	// ErrCodexAgentUnreadable reports a Codex agent file that cannot be loaded.
	ErrCodexAgentUnreadable = errors.New("harness: codex agent file cannot be loaded")
	// ErrCodexNilAgent reports a missing loaded agent.
	ErrCodexNilAgent = errors.New("harness: codex requires a loaded agent")
	// ErrCodexUnsupportedOutputFormat reports an output format other than json.
	ErrCodexUnsupportedOutputFormat = errors.New("harness: codex supports only the json output format")
)

// Setting keys of a Codex agent file that travel through their own channel
// (flags or stdin) rather than as -c overrides.
var codexOwnedKeys = map[string]bool{
	"name":                   true,
	"description":            true,
	"model":                  true,
	"sandbox_mode":           true,
	"developer_instructions": true,
}

// CodexContactUserTools returns the contact-user tools the invocation approves.
func CodexContactUserTools() []string {
	return []string{"ask_user_questions", "get_answered_questions"}
}

// BuildCodexArgs turns a loaded Codex agent plus the request into argv and stdin.
//
// argv never carries a newline or instruction text: a `.cmd` shim runs through
// `cmd /c` and the Windows command line is length limited. stdin is the agent's
// instructions, the system prompt and the prompt, joined by newlines. Approvals
// are disabled and every contact-user tool is approved, so the run is
// unattended. A request model overrides the agent's model.
func BuildCodexArgs(req SpawnRequest, agent *agentload.Agent) (args []string, stdin []byte, err error) {
	if agent == nil {
		return nil, nil, ErrCodexNilAgent
	}
	if req.OutputFormat != "" && req.OutputFormat != "json" {
		return nil, nil, ErrCodexUnsupportedOutputFormat
	}

	args = []string{"exec", "--json", "--skip-git-repo-check"}
	if req.WorkingDir != "" {
		args = append(args, "-C", req.WorkingDir)
	}

	model, sandbox, instructions := "", "", ""
	for _, setting := range agent.Settings {
		text, _ := setting.Value.(string)
		switch setting.Key {
		case "model":
			model = text
		case "sandbox_mode":
			sandbox = text
		case "developer_instructions":
			instructions = text
		}
	}
	if req.Model != "" {
		model = req.Model
	}
	if model != "" {
		args = append(args, "-m", model)
	}
	if sandbox != "" {
		args = append(args, "-s", sandbox)
	}

	fixed, fixedPaths := codexFixedOverrides()
	for _, kv := range fixed {
		args = append(args, "-c", kv)
	}
	for _, setting := range agent.Settings {
		if codexOwnedKeys[setting.Key] || fixedPaths[setting.Key] {
			continue
		}
		for _, kv := range codexOverrides(codexKey(setting.Key), setting.Value, fixedPaths) {
			args = append(args, "-c", kv)
		}
	}

	args = append(args, req.ExtraArgs...)
	args = append(args, "-")

	parts := make([]string, 0, 3)
	for _, part := range []string{strings.TrimSpace(instructions), req.SystemPrompt, req.Prompt} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return args, []byte(strings.Join(parts, "\n")), nil
}

// codexFixedOverrides returns the overrides every invocation sets itself and
// the config paths they cover, so a same-path agent setting is not emitted twice.
func codexFixedOverrides() ([]string, map[string]bool) {
	server := "mcp_servers." + CodexContactUserServer
	paths := map[string]bool{
		"approval_policy":                       true,
		server + ".default_tools_approval_mode": true,
	}
	fixed := []string{
		`approval_policy="never"`,
		server + `.default_tools_approval_mode="approve"`,
	}
	for _, tool := range CodexContactUserTools() {
		path := server + ".tools." + tool + ".approval_mode"
		paths[path] = true
		fixed = append(fixed, path+`="approve"`)
	}
	return fixed, paths
}

// NewCodex returns a Spawner for `codex exec`.
func NewCodex(executablePath string, opts ...Option) Spawner {
	cfg := &spawnerConfig{executablePath: executablePath}
	for _, opt := range opts {
		opt(cfg)
	}
	return &codexSpawner{cfg: cfg}
}

type codexSpawner struct {
	cfg *spawnerConfig
}

// Spawn implements Spawner. The agent is loaded from req.Agent.DefinitionPath
// (read-only) before anything starts. `codex exec` can exit zero on a failed
// turn, so the stream verdict is the success signal; on a non-zero exit that
// carries a stream-reported failure, both errors are returned.
func (s *codexSpawner) Spawn(ctx context.Context, req SpawnRequest) (Response, error) {
	agent, err := agentload.Load(req.Agent.DefinitionPath, string(formatid.CodexTOML))
	if err != nil {
		return Response{}, fmt.Errorf("%w: %w", ErrCodexAgentUnreadable, err)
	}

	cmd, err := ResolveExecutable(s.cfg.executablePath)
	if err != nil {
		return Response{}, err
	}

	args, stdin, err := BuildCodexArgs(req, agent)
	if err != nil {
		return Response{}, err
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = s.cfg.timeout
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	resp, runErr := Run(ctx, cmd, args, RunOptions{
		WorkingDir: req.WorkingDir,
		Env:        req.Env,
		Stdin:      stdin,
		Timeout:    timeout,
		Sink:       s.cfg.sinkFor(req),

		OnStdoutLine: req.OnStdoutLine,
	})
	if runErr != nil {
		if errors.Is(runErr, ErrNonZeroExit) {
			if _, parseErr := ParseCodexEnvelope(resp.Stdout); errors.Is(parseErr, ErrCodexStreamError) {
				return resp, fmt.Errorf("%w: %w", runErr, parseErr)
			}
		}
		return resp, runErr
	}

	text, err := ParseCodexEnvelope(resp.Stdout)
	if err != nil {
		return resp, err
	}

	protocol, err := ExtractProtocolJSON(text)
	if err != nil {
		return resp, err
	}

	resp.Text = text
	resp.Protocol = protocol
	return resp, nil
}
