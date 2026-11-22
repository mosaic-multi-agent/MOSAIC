package app

// promote_phase_source.go is the first phase of promoting one agent: validate the source path
// and harness, read the file through the decode funnel, apply the eligibility rule, derive the
// agent key and inspect the harness-side facts. It asks no question and writes nothing.

import (
	"fmt"
	"os"
	"path/filepath"

	"mosaic-deploy/internal/agentkey"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// promoteSource is a validated, decoded, eligible promote source.
type promoteSource struct {
	// path is the source file path as supplied by the caller.
	path string
	// harnessID names the harness the source was deployed for.
	harnessID string
	// module is the resolved harness module; the caller closes it.
	module domain.HarnessModule
	// canonical is the source in canonical Markdown form (identity for Markdown harnesses).
	canonical []byte
	// verdict is the eligibility verdict, carrying the role and version of the source.
	verdict EligibilityVerdict
	// key is the agent key derived from the source file name.
	key string
	// facts are the harness-side facts inspected from canonical.
	facts promoteSourceFacts
}

// loadPromoteSource validates path and harnessID, reads and decodes the source, and rejects
// an ineligible one. Decode-report gaps are added to the run's TODO. On success the caller
// owns src.module and must close it.
func (s *service) loadPromoteSource(path, harnessID string) (*promoteSource, error) {
	if path == "" {
		return nil, ErrPromoteFileRequired
	}

	// A missing or non-file path produces a domain error with the offending path in the
	// message. Any other stat failure is wrapped with path information.
	if info, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, fmt.Errorf("promote source %s: %w", path, ErrPromoteSourceNotFound)
		}
		return nil, fmt.Errorf("cannot access source file %s: %w", path, statErr)
	} else if info.IsDir() {
		return nil, fmt.Errorf("promote source %s: %w", path, ErrPromoteSourceNotFile)
	}

	// HarnessID has no interactive fallback; the caller must supply the harness it already knows.
	if harnessID == "" {
		return nil, ErrPromoteHarnessRequired
	}
	module, resolveErr := s.deps.Registry.Resolve(harnessID)
	if resolveErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrPromoteHarnessUnresolvable, resolveErr)
	}

	src, err := s.readPromoteSource(path, harnessID, module)
	if err != nil {
		_ = module.Close() // the original failure matters, not the close error
		return nil, err
	}
	return src, nil
}

// readPromoteSource decodes the source file through the funnel and runs the eligibility rule
// and the harness-side inspection.
func (s *service) readPromoteSource(path, harnessID string, module domain.HarnessModule) (*promoteSource, error) {
	// For Markdown harnesses the decode is the identity; for Codex TOML the file is
	// translated to canonical Markdown form.
	dr := readDeployedArtifact(path, domain.ArtifactAgent, module.Descriptor())
	if dr.ReadErr != nil {
		return nil, fmt.Errorf("reading source file %s: %w", path, dr.ReadErr)
	}
	if dr.DecodeErr != nil {
		return nil, fmt.Errorf("decoding source file %s: %w", path, dr.DecodeErr)
	}

	// The decode report is non-empty only on a successful decode that has informational
	// entries (e.g. a body-less source).
	var decodeRpt transform.Report
	mergeDecodeReport(&decodeRpt, dr.Report)
	for _, g := range decodeRpt.Gaps {
		s.deps.Todo.AddGap(g)
	}

	// The shared two-signal eligibility rule: both signals must be present.
	verdict := eligibleHarnessOnly(dr.Canonical)
	if !verdict.Eligible {
		return nil, fmt.Errorf("%w: %s: %s", ErrPromoteNotTransformed, path, verdict.Reason)
	}

	// The key is the file name without the harness's declared agent extension.
	agentExt := module.Descriptor().Extensions[domain.ArtifactAgent]
	if agentExt == "" {
		agentExt = ".md"
	}
	key := agentkey.FromFileNameExt(filepath.Base(path), agentExt)

	// Pure parse: tool entries and key presence, before any question is asked.
	facts, factsErr := inspectPromoteSource(dr.Canonical, module.Descriptor())
	if factsErr != nil {
		return nil, fmt.Errorf("%s: %w", path, factsErr)
	}

	return &promoteSource{
		path:      path,
		harnessID: harnessID,
		module:    module,
		canonical: dr.Canonical,
		verdict:   verdict,
		key:       key,
		facts:     facts,
	}, nil
}
