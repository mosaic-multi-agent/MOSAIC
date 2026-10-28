package main

// usageingest_wiring.go holds the Runner's GHCP CLI usage ingest wiring: the
// adapter decorator that runs a final ingest after each harness process exits
// and the session decorator that records the starting offset of a new run and
// catches up on the not-yet-ingested usage of a resumed run.
//
// Ingest is strictly optional: nothing here changes what an adapter or a
// session returns; every failure is swallowed into a debug-log diagnostic.

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"mosaic-common/otelingest"
	"mosaic-common/otelingest/diag"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/session"
)

const (
	orchestratorStreamFile = "00_orchestrator_events.jsonl"
	invocationStreamFile   = "03_events.jsonl"
)

// ghcpCLIHarness is the closed capability set of *ghcpcli.GHCPCLIAdapter.
type ghcpCLIHarness interface {
	domain.HarnessAdapter
	domain.RawInvoker
	domain.ExecutableRevealer
	runIDReporter
}

// usageIngestConfig is shared by the adapter decorator and the session decorator of one run.
type usageIngestConfig struct {
	RunDir      string // <workspace>/OrchestrationLogs/{run_id}
	OTelPath    string // from the Runner's own environment; "" = unset
	StartOffset *int64 // set by the session decorator for a new run
	Debug       domain.DebugLogger
	Ingest      func(otelingest.Request) otelingest.Result // seam; production = otelingest.Ingest
}

// reportedNotices remembers which once-per-run diagnostics were already logged,
// keyed on run directory and diagnostic kind. It outlives a single wiring so a
// resumed or rebuilt session of the same run does not repeat a diagnostic.
var reportedNotices sync.Map

// reportOnce logs the diagnostic of kind for the run the first time only.
func (c *usageIngestConfig) reportOnce(kind diag.Kind, message string) {
	if _, dup := reportedNotices.LoadOrStore(c.RunDir+"|"+string(kind), struct{}{}); dup {
		return
	}
	c.Debug.Log(domain.EventRunnerError, "usage ingest: "+string(kind)+": "+message,
		domain.F("run_dir", c.RunDir))
}

// configured reports whether an OTel path is available; when it is not, the
// unset diagnostic is logged once for the run.
func (c *usageIngestConfig) configured() bool {
	if c.OTelPath == "" {
		c.reportOnce(diag.KindVariableUnset, otelingest.EnvVar+" is not set; usage is not ingested")
		return false
	}
	return true
}

// finalIngest runs one final ingest and logs its diagnostics. A nil scope is the
// unscoped catch-up; process ingests always pass a non-empty scope.
func (c *usageIngestConfig) finalIngest(scope []string, startOffset *int64) {
	res := c.Ingest(otelingest.Request{
		RunDir:      c.RunDir,
		OTelPath:    c.OTelPath,
		Final:       true,
		StartOffset: startOffset,
		Scope:       scope,
	})
	for _, d := range res.Diagnostics {
		if d.Kind == diag.KindFileMissing {
			c.reportOnce(d.Kind, d.Message)
			continue
		}
		c.Debug.Log(domain.EventRunnerError, "usage ingest: "+string(d.Kind)+": "+d.Message,
			domain.F("run_dir", c.RunDir))
	}
	if res.Err != nil {
		c.Debug.Log(domain.EventRunnerError, "usage ingest failed: "+res.Err.Error(),
			domain.F("run_dir", c.RunDir))
	}
}

// sessionIDsIn returns the distinct session_id values of a MOSAIC event stream in
// first-seen order; an unreadable or absent stream yields none.
func sessionIDsIn(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var ids []string
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var env struct {
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal(sc.Bytes(), &env) != nil || env.SessionID == "" || seen[env.SessionID] {
			continue
		}
		seen[env.SessionID] = true
		ids = append(ids, env.SessionID)
	}
	return ids
}

// usageIngestAdapter runs a final ingest after each harness process exits and
// forwards every other capability of the wrapped adapter unchanged.
type usageIngestAdapter struct {
	ghcpCLIHarness
	cfg *usageIngestConfig
}

func (a *usageIngestAdapter) Invoke(ctx context.Context, ref domain.AgentReference, req domain.ProtocolRequest) (domain.ProtocolResponse, error) {
	resp, err := a.ghcpCLIHarness.Invoke(ctx, ref, req)
	if a.cfg.configured() && req.AgentInstanceID != "" {
		stream := filepath.Join(a.cfg.RunDir, req.AgentInstanceID, invocationStreamFile)
		if scope := sessionIDsIn(stream); len(scope) > 0 {
			a.cfg.finalIngest(scope, a.cfg.StartOffset)
		}
	}
	return resp, err
}

func (a *usageIngestAdapter) InvokeRaw(ctx context.Context, ref domain.AgentReference, payload []byte) ([]byte, error) {
	stream := filepath.Join(a.cfg.RunDir, orchestratorStreamFile)
	before := map[string]bool{}
	for _, id := range sessionIDsIn(stream) {
		before[id] = true
	}
	reply, err := a.ghcpCLIHarness.InvokeRaw(ctx, ref, payload)
	if a.cfg.configured() {
		var scope []string
		for _, id := range sessionIDsIn(stream) {
			if !before[id] {
				scope = append(scope, id)
			}
		}
		if len(scope) > 0 {
			a.cfg.finalIngest(scope, a.cfg.StartOffset)
		}
	}
	return reply, err
}

func newGHCPCLIUsageIngest(inner ghcpCLIHarness, cfg *usageIngestConfig) ghcpCLIHarness {
	return &usageIngestAdapter{ghcpCLIHarness: inner, cfg: cfg}
}

// runUsageSession records the starting offset of a new run and catches up the
// usage of a resumed run, both before the wrapped session starts any process.
type runUsageSession struct {
	inner session.Session
	cfg   *usageIngestConfig
}

func newRunUsageSession(inner session.Session, cfg *usageIngestConfig) session.Session {
	return &runUsageSession{inner: inner, cfg: cfg}
}

func (s *runUsageSession) Start(ctx context.Context, cfg domain.RunConfig) (domain.RunOutcome, error) {
	if s.cfg.configured() {
		if cfg.IsNewRun {
			s.recordStartOffset()
		} else {
			s.cfg.finalIngest(nil, nil)
		}
	}
	return s.inner.Start(ctx, cfg)
}

// recordStartOffset stores the OTel file's current size as the run's first cursor;
// a missing file starts the run at 0 and is reported once.
func (s *runUsageSession) recordStartOffset() {
	var offset int64
	info, err := os.Stat(s.cfg.OTelPath)
	switch {
	case err == nil:
		offset = info.Size()
	case os.IsNotExist(err):
		s.cfg.reportOnce(diag.KindFileMissing, "OTel file does not exist")
	default:
		s.cfg.Debug.Log(domain.EventRunnerError, "usage ingest: cannot stat OTel file: "+err.Error())
		return
	}
	s.cfg.StartOffset = &offset
}

// withGHCPCLIUsageIngest decorates the GHCP CLI adapter when runFolder is an
// Orchestration-{run_id} folder; otherwise the adapter is returned undecorated.
// The OTel path is read once from the Runner's own environment.
func withGHCPCLIUsageIngest(adapter ghcpCLIHarness, runFolder string, logger domain.DebugLogger) domain.HarnessAdapter {
	runID, ok := domain.ParseRunFolder(filepath.Base(runFolder))
	if !ok {
		return adapter
	}
	cfg := &usageIngestConfig{
		RunDir:   filepath.Join(filepath.Dir(runFolder), "OrchestrationLogs", runID),
		OTelPath: strings.TrimSpace(os.Getenv(otelingest.EnvVar)),
		Debug:    logger,
		Ingest:   otelingest.Ingest,
	}
	return newGHCPCLIUsageIngest(adapter, cfg)
}

// usageIngestOf returns the config of h when h is the GHCP CLI usage decorator; nil otherwise.
func usageIngestOf(h domain.HarnessAdapter) *usageIngestConfig {
	if d, ok := h.(*usageIngestAdapter); ok {
		return d.cfg
	}
	return nil
}

// newRunLogConfig builds the runLogConfig that both production call sites
// (main.go and wiring_stub.go) pass to newRunLifecycleSession.
func newRunLogConfig(adapter domain.HarnessAdapter, runFolder, harnessID string, logger domain.DebugLogger, clock domain.Clock) runLogConfig {
	return runLogConfig{RunFolder: runFolder, HarnessID: harnessID, Debug: logger, Clock: clock, UsageIngest: usageIngestOf(adapter)}
}
