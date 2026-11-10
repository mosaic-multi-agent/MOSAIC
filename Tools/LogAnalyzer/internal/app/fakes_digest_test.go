package app_test

// In-memory digest port fake and a scenario builder for the digest-source tests.

import (
	"context"
	"fmt"
	"path"
	"time"

	"mosaic-common/interaction"
	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// ---------------------------------------------------------------------------
// MockDigestStore implements domain.DigestStore.
// ---------------------------------------------------------------------------

// MockDigestStore serves scripted digests per file path. Discover answers come
// from the discover map keyed by the probed path.
type MockDigestStore struct {
	discover map[string][]string
	digests  map[string]domain.Digest
	findings map[string][]domain.Finding

	discoverCalls []string
	readCalls     []string

	// existing holds output paths that already exist; Write refuses them unless overwrite is set.
	existing map[string]bool
	// writes records every successful Write, in call order.
	writes []digestWrite
}

// digestWrite is one successful DigestStore.Write call.
type digestWrite struct {
	path      string
	d         domain.Digest
	overwrite bool
}

func newMockDigestStore() *MockDigestStore {
	return &MockDigestStore{
		discover: make(map[string][]string),
		digests:  make(map[string]domain.Digest),
		findings: make(map[string][]domain.Finding),
		existing: make(map[string]bool),
	}
}

func (m *MockDigestStore) Discover(p string) []string {
	m.discoverCalls = append(m.discoverCalls, p)
	return m.discover[p]
}

func (m *MockDigestStore) Read(ctx context.Context, p string) (domain.Digest, []domain.Finding, error) {
	m.readCalls = append(m.readCalls, p)
	if err := ctx.Err(); err != nil {
		return domain.Digest{}, nil, err
	}
	return m.digests[p], m.findings[p], nil
}

func (m *MockDigestStore) Write(ctx context.Context, p string, d domain.Digest, overwrite bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.existing[p] && !overwrite {
		return fmt.Errorf("write %s: %w", p, domain.ErrDigestExists)
	}
	m.writes = append(m.writes, digestWrite{path: p, d: d, overwrite: overwrite})
	return nil
}

// ---------------------------------------------------------------------------
// Constants and builders.
// ---------------------------------------------------------------------------

const (
	dgLogsPath = "/digest-test/logs"
	dgModel    = domain.ModelID("model-a")

	dgRunA = "20260101T000000Z-aaaa"
	dgRunB = "20260102T000000Z-bbbb"
	dgRunC = "20251231T000000Z-cccc" // sorts before dgRunA
)

var (
	dgT1 = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	dgT2 = dgT1.Add(time.Hour)
)

// dgRun builds a digest run with one orchestrator usage entry of the given input tokens.
func dgRun(id string, input int64) domain.RunAggregate {
	return domain.RunAggregate{
		Run: domain.NamedRun(id),
		Orchestrator: domain.ActorAggregate{
			Actor: domain.Orchestrator(),
			Invocations: []domain.InvocationUsage{
				{Model: dgModel, Usage: domain.TokenUsage{Input: domain.Tokens(input)}},
			},
		},
	}
}

// dgBucket builds the unattributable-bucket aggregate with the given input tokens.
func dgBucket(input int64) *domain.RunAggregate {
	r := dgRun("", input)
	r.Run = domain.UnattributableRun()
	return &r
}

// dgDigest builds a digest written at the given time holding the given runs.
func dgDigest(writtenAt time.Time, runs ...domain.RunAggregate) domain.Digest {
	return domain.Digest{
		Header:    domain.DigestHeader{Version: "1.0.0", WrittenAt: writtenAt},
		Aggregate: domain.Aggregate{Runs: runs},
	}
}

// dgFinding builds a carried finding scoped to the given run.
func dgFinding(kind domain.FindingKind, run domain.RunRef) domain.Finding {
	return domain.Finding{Kind: kind, Severity: domain.SeverityWarning, Run: run}
}

// digestItem is one scripted digest file.
type digestItem struct {
	name     string
	d        domain.Digest
	findings []domain.Finding
}

// ---------------------------------------------------------------------------
// digestEnv: a service built on fakes with logs and digests scripted per path.
// ---------------------------------------------------------------------------

type digestEnv struct {
	logSource *fakeLogSource
	reader    *fakeEventReader
	pricing   *fakePricingStore
	digests   *MockDigestStore

	logsPath    string
	hasLogs     bool
	unusable    map[string]bool
	logRuns     []domain.RunEntry
	logBucket   *domain.RunEntry
	interaction domain.Interaction

	enumFindings []domain.Finding // findings Enumerate returns for the logs root
	clock        *fakeClock
	toolVersion  string
}

func newDigestEnv() *digestEnv {
	e := &digestEnv{
		reader:      newFakeEventReader(),
		pricing:     newFakePricingStore(),
		digests:     newMockDigestStore(),
		logsPath:    dgLogsPath,
		unusable:    make(map[string]bool),
		interaction: &alwaysSkippingInteraction{},
		clock:       newFakeClock(),
	}
	e.pricing.table = e.pricing.table.With(flatPricing(dgModel))
	e.logSource = &fakeLogSource{
		classifyFunc: func(p string) domain.Source {
			switch {
			case e.hasLogs && p == e.logsPath:
				return domain.Source{Kind: domain.SourceLogsRoot, Path: p}
			case e.unusable[p]:
				return domain.Source{Kind: domain.SourceUnusable, Path: p, Reason: "no log structure"}
			}
			return domain.Source{Kind: domain.SourceNotFound, Path: p}
		},
		enumerateFunc: func(src domain.Source) (domain.Inventory, []domain.Finding) {
			if src.Kind != domain.SourceLogsRoot {
				return domain.Inventory{Source: src}, nil
			}
			return domain.Inventory{Source: src, Runs: e.logRuns, Unattributable: e.logBucket}, e.enumFindings
		},
	}
	return e
}

func (e *digestEnv) service() *app.Service {
	return app.New(app.Deps{
		Source:      e.logSource,
		Reader:      e.reader,
		Pricing:     e.pricing,
		Digests:     e.digests,
		Clock:       e.clock,
		Interaction: e.interaction,
		WorkDir:     "/workdir",
		ToolVersion: e.toolVersion,
	})
}

// addLogRun registers a log run with one orchestrator turn of the given input tokens.
func (e *digestEnv) addLogRun(id string, input int64) {
	e.hasLogs = true
	dir := e.logsPath + "/" + id
	orch := dir + "/" + "00_orchestrator_events.jsonl"
	e.logRuns = append(e.logRuns, domain.RunEntry{Run: domain.NamedRun(id), Dir: dir, OrchestratorFile: orch})
	e.reader.addEvents(orch,
		appTestTurnEvent(dgModel, domain.TokenUsage{Input: domain.Tokens(input)}),
		appTestRunEndEvent(),
	)
}

// addLogBucket registers an unknown-run log bucket with one turn of the given input tokens.
func (e *digestEnv) addLogBucket(input int64) {
	e.hasLogs = true
	dir := e.logsPath + "/unknown-run"
	orch := dir + "/00_orchestrator_events.jsonl"
	e.logBucket = &domain.RunEntry{Run: domain.UnattributableRun(), Dir: dir, OrchestratorFile: orch}
	e.reader.addEvents(orch, appTestTurnEvent(dgModel, domain.TokenUsage{Input: domain.Tokens(input)}))
}

// digestFile scripts a single digest file at p (Discover(p) == [p]) and marks p unusable as logs.
func (e *digestEnv) digestFile(p string, d domain.Digest, findings ...domain.Finding) {
	e.unusable[p] = true
	e.digests.discover[p] = []string{p}
	e.digests.digests[p] = d
	e.digests.findings[p] = findings
}

// digestDir scripts the digests found directly inside dir, in the given (sorted) order.
// The directory is unusable as logs unless logs were added at the same path.
func (e *digestEnv) digestDir(dir string, items ...digestItem) {
	if !(e.hasLogs && dir == e.logsPath) {
		e.unusable[dir] = true
	}
	var files []string
	for _, it := range items {
		f := path.Join(dir, it.name)
		files = append(files, f)
		e.digests.digests[f] = it.d
		e.digests.findings[f] = it.findings
	}
	e.digests.discover[dir] = files
}

// withInteraction swaps the Interaction used by service().
func (e *digestEnv) withInteraction(ia interaction.Interaction) *digestEnv {
	e.interaction = ia
	return e
}

// ---------------------------------------------------------------------------
// Result helpers.
// ---------------------------------------------------------------------------

func dgRunIDs(r domain.Report) []string {
	var ids []string
	for _, run := range r.Runs {
		ids = append(ids, run.Run.ID)
	}
	return ids
}

// dgInput returns the named run's total input tokens (0, false when absent).
func dgInput(r domain.Report, id string) (int64, bool) {
	run, ok := r.FindRun(id)
	if !ok {
		return 0, false
	}
	v, _ := run.Totals.Tokens.Input.Value()
	return v, true
}

// dgFindings returns the report's quality findings of one kind.
func dgFindings(r domain.Report, kind domain.FindingKind) []domain.Finding {
	var out []domain.Finding
	for _, f := range r.Quality.Findings {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}
