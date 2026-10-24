package cli_test

// Fakes and a scenario builder for the export command tests: an in-memory
// digest store, a prompt-counting Interaction, and a logs source scripted with
// a fixed set of runs.

import (
	"bytes"
	"context"
	"fmt"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/cli"
	"mosaic-log-analyzer/internal/domain"
)

const (
	exportLogsPath     = "/export-logs"
	exportUnusablePath = "/export-unusable"
	exportOutPath      = "out.digest.json"

	exportRunOne = "20260101T000000Z-aaaa"
	exportRunTwo = "20260102T000000Z-bbbb"
)

// ---------------------------------------------------------------------------
// MockDigestStore implements domain.DigestStore.
// ---------------------------------------------------------------------------

// digestWrite is one successful DigestStore.Write call.
type digestWrite struct {
	path      string
	d         domain.Digest
	overwrite bool
}

// MockDigestStore records writes and refuses existing paths unless overwriting.
type MockDigestStore struct {
	existing map[string]bool
	writeErr error
	writes   []digestWrite
}

func newMockDigestStore() *MockDigestStore {
	return &MockDigestStore{existing: make(map[string]bool)}
}

func (m *MockDigestStore) Discover(string) []string { return nil }

func (m *MockDigestStore) Read(ctx context.Context, _ string) (domain.Digest, []domain.Finding, error) {
	return domain.Digest{}, nil, ctx.Err()
}

func (m *MockDigestStore) Write(ctx context.Context, p string, d domain.Digest, overwrite bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.writeErr != nil {
		return m.writeErr
	}
	if m.existing[p] && !overwrite {
		return fmt.Errorf("write %s: %w", p, domain.ErrDigestExists)
	}
	m.writes = append(m.writes, digestWrite{path: p, d: d, overwrite: overwrite})
	return nil
}

// ---------------------------------------------------------------------------
// promptCountingInteraction implements domain.Interaction.
// ---------------------------------------------------------------------------

// promptCountingInteraction skips every question like alwaysSkippingInteraction
// and counts how many were asked.
type promptCountingInteraction struct {
	alwaysSkippingInteraction
	asked int
}

func (i *promptCountingInteraction) SelectOne(ctx context.Context, q interaction.ChoiceQuestion) (interaction.ChoiceAnswer, error) {
	i.asked++
	return i.alwaysSkippingInteraction.SelectOne(ctx, q)
}

func (i *promptCountingInteraction) SelectMany(ctx context.Context, q interaction.ChoiceQuestion) (interaction.MultiChoiceAnswer, error) {
	i.asked++
	return i.alwaysSkippingInteraction.SelectMany(ctx, q)
}

func (i *promptCountingInteraction) AskText(ctx context.Context, q interaction.TextQuestion) (interaction.TextAnswer, error) {
	i.asked++
	return i.alwaysSkippingInteraction.AskText(ctx, q)
}

func (i *promptCountingInteraction) Confirm(ctx context.Context, q interaction.Question) (interaction.ConfirmAnswer, error) {
	i.asked++
	return i.alwaysSkippingInteraction.Confirm(ctx, q)
}

// ---------------------------------------------------------------------------
// exportEnv: a service on fakes with a scripted logs root.
// ---------------------------------------------------------------------------

type exportEnv struct {
	reader     *fakeEventReader
	digests    *MockDigestStore
	prompts    *promptCountingInteraction
	runs       []domain.RunEntry
	bucket     *domain.RunEntry
	classified []string
}

// newExportEnv scripts a logs root holding the given runs, each with one
// orchestrator turn.
func newExportEnv(runIDs ...string) *exportEnv {
	e := &exportEnv{
		reader:  newFakeEventReader(),
		digests: newMockDigestStore(),
		prompts: &promptCountingInteraction{},
	}
	for _, id := range runIDs {
		dir := exportLogsPath + "/" + id
		orch := dir + "/00_orchestrator_events.jsonl"
		e.runs = append(e.runs, domain.RunEntry{Run: domain.NamedRun(id), Dir: dir, OrchestratorFile: orch})
		e.reader.addEvents(orch,
			cliTestTurnEvent("model-a", domain.TokenUsage{Input: domain.Tokens(1000)}),
			cliTestRunEndEvent(),
		)
	}
	return e
}

// withBucket adds an unattributable bucket with one turn.
func (e *exportEnv) withBucket() *exportEnv {
	dir := exportLogsPath + "/unknown-run"
	orch := dir + "/00_orchestrator_events.jsonl"
	e.bucket = &domain.RunEntry{Run: domain.UnattributableRun(), Dir: dir, OrchestratorFile: orch}
	e.reader.addEvents(orch, cliTestTurnEvent("model-a", domain.TokenUsage{Input: domain.Tokens(50)}))
	return e
}

func (e *exportEnv) service() *app.Service {
	src := &fakeLogSource{
		classifyFunc: func(p string) domain.Source {
			e.classified = append(e.classified, p)
			switch p {
			case exportLogsPath:
				return domain.Source{Kind: domain.SourceLogsRoot, Path: p}
			case exportUnusablePath:
				return domain.Source{Kind: domain.SourceUnusable, Path: p, Reason: "no log structure"}
			}
			return domain.Source{Kind: domain.SourceNotFound, Path: p}
		},
		enumerateFunc: func(s domain.Source) (domain.Inventory, []domain.Finding) {
			if s.Kind != domain.SourceLogsRoot {
				return domain.Inventory{Source: s}, nil
			}
			return domain.Inventory{Source: s, Runs: e.runs, Unattributable: e.bucket}, nil
		},
	}
	return app.New(app.Deps{
		Source:      src,
		Reader:      e.reader,
		Pricing:     newFakePricingStore(),
		Digests:     e.digests,
		Clock:       newFakeClock(),
		Interaction: e.prompts,
		WorkDir:     "/workdir",
		ToolVersion: "9.9.9",
	})
}

// execute runs the CLI with args and returns the exit code, stdout and stderr.
func (e *exportEnv) execute(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = cli.Execute(cli.Options{Service: e.service(), Stdout: &out, Stderr: &errOut}, args)
	return code, out.String(), errOut.String()
}
