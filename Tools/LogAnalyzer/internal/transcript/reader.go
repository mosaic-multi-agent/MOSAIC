// Package transcript is the I/O adapter that reads the harness session
// transcripts copied into a log source and returns only the facts the
// transcript integrity checks need.
package transcript

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

const (
	orchestratorRaw     = "00_orchestrator_session.raw"
	orchestratorSidecar = "00_orchestrator_session.meta.json"
	agentRaw            = "04_session.raw"
	agentSidecar        = "04_session.meta.json"
)

// Reader implements domain.TranscriptSource.
type Reader struct{}

// NewReader returns a Reader.
func NewReader() *Reader { return &Reader{} }

// Transcripts implements domain.TranscriptSource.
func (r *Reader) Transcripts(ctx context.Context, inv domain.Inventory) []domain.TranscriptFacts {
	if inv.Source.Kind != domain.SourceLogsRoot && inv.Source.Kind != domain.SourceSingleRun {
		return nil
	}
	runs := append([]domain.RunEntry(nil), inv.Runs...)
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].Run.ID < runs[j].Run.ID })

	var out []domain.TranscriptFacts
	for _, run := range runs {
		if run.Run.Kind != domain.RunNamed {
			continue
		}
		if ctx.Err() != nil {
			return out
		}
		if f, ok := readTranscript(run.Run, domain.StreamOrchestrator, "",
			filepath.Join(run.Dir, orchestratorRaw), filepath.Join(run.Dir, orchestratorSidecar)); ok {
			out = append(out, f)
		}
		agents := append([]domain.AgentEntry(nil), run.Agents...)
		sort.SliceStable(agents, func(i, j int) bool { return agents[i].InstanceHint < agents[j].InstanceHint })
		for _, a := range agents {
			if ctx.Err() != nil {
				return out
			}
			if f, ok := readTranscript(run.Run, domain.StreamAgentInstance, a.InstanceHint,
				filepath.Join(a.Dir, agentRaw), filepath.Join(a.Dir, agentSidecar)); ok {
				out = append(out, f)
			}
		}
	}
	return out
}

// readTranscript reads one raw transcript; ok is false when it yields no facts.
func readTranscript(run domain.RunRef, owner domain.StreamKind, hint domain.AgentInstanceID, rawPath, sidecarPath string) (domain.TranscriptFacts, bool) {
	format, ok := readFormat(sidecarPath)
	if !ok {
		return domain.TranscriptFacts{}, false
	}
	file, err := os.Open(rawPath)
	if err != nil {
		return domain.TranscriptFacts{}, false
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		return domain.TranscriptFacts{}, false
	}

	f := domain.TranscriptFacts{Run: run, Owner: owner, InstanceHint: hint, Path: rawPath, Format: format}
	wantPrompt := owner == domain.StreamAgentInstance
	switch format {
	case domain.TranscriptClaudeCodeJSONL:
		f.AssistantIDs, f.FirstPrompt, f.HasFirstPrompt = readClaudeJSONL(file, wantPrompt)
	case domain.TranscriptOpenCodeMessages:
		if wantPrompt {
			f.FirstPrompt, f.HasFirstPrompt = readOpenCodeFirstPrompt(file)
		}
	}
	return f, true
}

// readFormat returns the known native format named by a sidecar.
func readFormat(path string) (domain.TranscriptFormat, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var meta struct {
		NativeFormat string `json:"native_format"`
	}
	if json.Unmarshal(raw, &meta) != nil {
		return "", false
	}
	switch f := domain.TranscriptFormat(meta.NativeFormat); f {
	case domain.TranscriptClaudeCodeJSONL, domain.TranscriptOpenCodeMessages:
		return f, true
	}
	return "", false
}

var _ domain.TranscriptSource = (*Reader)(nil)
