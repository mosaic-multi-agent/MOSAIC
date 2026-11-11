package app_test

// transform_mcp_deny_question_test.go specifies that an ambiguous OpenCode name that is denied
// is asked like an allowed one, and that its deny is never turned into a grant: GHCP CLI cannot
// express a deny, so the name is omitted whatever the answer.

import (
	"slices"
	"strings"
	"testing"

	"mosaic-deploy/internal/app/interactiontest"
	"mosaic-deploy/internal/domain"
)

const (
	trDenyAgentKey = "mcp-deny-agent"
	trSplitDenied  = "denied_get_item"
)

// trDenyRun is an OpenCode to GHCP CLI run over the agent that denies an ambiguous name.
func trDenyRun(t *testing.T, script *interactiontest.Stub) *trRun {
	t.Helper()
	r := trNew(t, script, hOpen, hCLI)
	r.put(hOpen, trDenyAgentKey, trOpenCodeFixture(t, "opencode-agents", trDenyAgentKey+".md"))
	return r
}

func TestTransformMcpDeny_AmbiguousDeniedNameIsAskedOnceLikeAnAllowedName(t *testing.T) {
	r := trDenyRun(t, nil)

	r.mustRun(r.request())

	if got, want := r.spy.splitSubjects(), []string{trSplitDenied}; !slices.Equal(got, want) {
		t.Errorf("split questions = %q, want %q", got, want)
	}
}

func TestTransformMcpDeny_SkippedDeniedNameIsNotGranted(t *testing.T) {
	r := trDenyRun(t, nil) // the stub skips an unscripted question

	r.mustRun(r.request())

	out := r.read(r.dest(trDenyAgentKey))
	if strings.Contains(out, "denied") {
		t.Errorf("a skipped denied name must not be granted:\n%s", out)
	}
	if !strings.Contains(out, "contact-user/*") {
		t.Errorf("the allowed server must still be written:\n%s", out)
	}
}

func TestTransformMcpDeny_AnsweredDeniedNameIsOmittedBecauseTheTargetCannotExpressDeny(t *testing.T) {
	script := interactiontest.NewBuilder().
		AnswerSelectOne(domain.QMCPNameSplit, trSplitDenied, "denied").Build()
	r := trDenyRun(t, script)

	r.mustRun(r.request())

	out := r.read(r.dest(trDenyAgentKey))
	if strings.Contains(out, "denied") {
		t.Errorf("a denied name must never be granted in the target, found it:\n%s", out)
	}
	if !strings.Contains(out, "contact-user/*") {
		t.Errorf("the allowed server must still be written:\n%s", out)
	}
}
