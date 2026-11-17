// OpenCode V2 argument builder, child environment and spawner.
package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrOpenCodeV2EmptyAgentIdentifier: req.Agent.Identifier is "".
var ErrOpenCodeV2EmptyAgentIdentifier = errors.New("harness: opencode-v2 requires a non-empty agent identifier")

// ErrOpenCodeV2UnsupportedOutputFormat: req.OutputFormat is neither "" nor "json".
var ErrOpenCodeV2UnsupportedOutputFormat = errors.New("harness: opencode-v2 supports only the json output format")

// BuildOpenCodeV2Args builds one headless V2 dispatch. Pure.
//
// The arguments are always `run --agent <id> --format json --auto
// --standalone`, then `--model <model>` when a model is supplied (a variant
// travels inside the model as `model#variant`), then ExtraArgs. V2 has no
// --dir or --variant. The prompt is delivered on stdin.
func BuildOpenCodeV2Args(req SpawnRequest) (args []string, stdin []byte, err error) {
	if req.Agent.Identifier == "" {
		return nil, nil, ErrOpenCodeV2EmptyAgentIdentifier
	}
	if req.OutputFormat != "" && req.OutputFormat != "json" {
		return nil, nil, ErrOpenCodeV2UnsupportedOutputFormat
	}

	args = []string{"run", "--agent", req.Agent.Identifier, "--format", "json", "--auto", "--standalone"}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	args = append(args, req.ExtraArgs...)

	content := req.Prompt
	if req.SystemPrompt != "" {
		content = req.SystemPrompt + "\n" + req.Prompt
	}
	if content != "" {
		stdin = []byte(content)
	}
	return args, stdin, nil
}

// OpenCodeV2Env returns the environment entries to pass as RunOptions.Env for
// a V2 child so that its PWD equals its working directory.
//
// V2 derives its project from PWD, so a stale inherited PWD would make it
// resolve the wrong project. Run can override but not remove an inherited
// variable; the PWD entry is therefore appended last. When the directory
// cannot be determined, env is returned unchanged.
func OpenCodeV2Env(workingDir string, env []string) []string {
	dir := workingDir
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return env
		}
		dir = cwd
	} else {
		abs, err := filepath.Abs(workingDir)
		if err != nil {
			return env
		}
		dir = abs
	}

	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if key == "PWD" || (runtime.GOOS == "windows" && strings.EqualFold(key, "PWD")) {
			continue
		}
		out = append(out, entry)
	}
	return append(out, "PWD="+dir)
}

type openCodeV2Spawner struct{ cfg *spawnerConfig }

// NewOpenCodeV2 returns a Spawner for OpenCode V2. Options: WithTimeout, WithSink.
func NewOpenCodeV2(executablePath string, opts ...Option) Spawner {
	cfg := &spawnerConfig{executablePath: executablePath}
	for _, opt := range opts {
		opt(cfg)
	}
	return &openCodeV2Spawner{cfg: cfg}
}

// Spawn implements Spawner. Unlike the V1 path it keeps the Response (stdout,
// stderr, exit code) on a verdict failure, because V2 reports its failure
// reason in a stdout error event.
func (s *openCodeV2Spawner) Spawn(ctx context.Context, req SpawnRequest) (Response, error) {
	cmd, err := ResolveExecutable(s.cfg.executablePath)
	if err != nil {
		return Response{}, err
	}
	args, stdin, err := BuildOpenCodeV2Args(req)
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
		WorkingDir:   req.WorkingDir,
		Env:          OpenCodeV2Env(req.WorkingDir, req.Env),
		Stdin:        stdin,
		Timeout:      timeout,
		Sink:         s.cfg.sink,
		OnStdoutLine: req.OnStdoutLine,

		NoOutputTimeout: req.NoOutputTimeout,
	})

	text, err := OpenCodeV2Verdict(resp, runErr)
	if err != nil {
		return resp, err
	}
	resp.Text = text

	protocol, err := ExtractProtocolJSON(text)
	if err != nil {
		return resp, err
	}
	resp.Protocol = protocol
	return resp, nil
}
