package cli_test

// transform_refusal_test.go covers the exit code and output of a transform between the two GHCP
// targets that the service refused because the workspace holds both of them.

import (
	"fmt"
	"strings"
	"testing"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/cli"
	"mosaic-deploy/internal/domain"
)

func TestTransformCmd_PairResultWithRemovalsAndReplacements_KeepsExitCodeZero(t *testing.T) {
	res := app.TransformHarnessResult{Transformed: 1,
		Files: []app.TransformFileOutcome{{Kind: domain.ArtifactAgent, Key: "a",
			Action: app.PlanWriteCreate, Status: app.StatusTransformed}},
		Removals:     []app.TransformFileChange{{Kind: domain.ArtifactAgent, Key: "a", Path: ".github/agents/a.md", Reason: "r"}},
		Replacements: []app.TransformFileChange{{Kind: domain.ArtifactHook, Key: "h", Path: ".github/hooks/mosaic-logger.json", Reason: "r"}},
	}

	code, _, _ := runTransform(&spyService{transformResp: res}, transformArgs("--yes")...)

	if code != cli.ExitSuccess {
		t.Errorf("exit code = %d, want ExitSuccess: removals and replacements are not skips", code)
	}
}

func TestTransformCmd_RefusedGhcpPair_ExitsWithFailureAndExplainsOnStderr(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			svc := &spyService{transformErr: fmt.Errorf("transform: %w", app.ErrGhcpBothTargets)}

			code, out, errOut := runTransform(svc, transformArgs("--output", format)...)

			if code != cli.ExitFailure {
				t.Errorf("exit code = %d, want ExitFailure", code)
			}
			if strings.TrimSpace(out) != "" {
				t.Errorf("stdout = %q, want nothing for a refused run without a result", out)
			}
			if !strings.Contains(errOut, app.ErrGhcpBothTargets.Error()) {
				t.Errorf("stderr lacks the refusal:\n%s", errOut)
			}
		})
	}
}
