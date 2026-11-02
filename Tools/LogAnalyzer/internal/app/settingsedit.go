package app

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"mosaic-log-analyzer/internal/domain"
)

// SettingsEditResult is what MarkUserInteractionTool returns.
type SettingsEditResult struct {
	Report     domain.Report // recomputed; Quality copied from the input report
	ConfigPath string        // the settings file written
	// ConfigLevel is the level of that file.
	ConfigLevel domain.ConfigLevel
	Tool        string
	Harnesses   []domain.Harness // harnesses the tool was saved under, sorted
}

// ErrSettingsUnavailable: Deps.Settings is nil.
var ErrSettingsUnavailable = errors.New("no settings store configured")

// MarkUserInteractionTool saves tool as user-interaction for every harness and
// recomputes the report from the session's cache. Input is validated before
// anything is saved; if a save fails the session settings stay unchanged.
func (s *Service) MarkUserInteractionTool(ctx context.Context, sess *Session, r domain.Report, tool string, harnesses []domain.Harness) (SettingsEditResult, error) {
	if s.deps.Settings == nil {
		return SettingsEditResult{}, ErrSettingsUnavailable
	}
	if sess == nil {
		return SettingsEditResult{}, errors.New("no session to recompute")
	}
	if len(harnesses) == 0 {
		return SettingsEditResult{}, fmt.Errorf("%w: no harness given", domain.ErrInvalidHarness)
	}
	sorted := sortedUniqueHarnesses(harnesses)
	updated := sess.Settings()
	for _, h := range sorted {
		next, err := updated.WithUserInteractionTool(h, tool)
		if err != nil {
			return SettingsEditResult{}, err
		}
		updated = next
	}
	level, ok, err := s.askConfigLevel(ctx, s.deps.Settings.Paths())
	if err != nil {
		return SettingsEditResult{}, err
	}
	if !ok {
		return SettingsEditResult{Report: r, Tool: tool}, nil
	}
	var file domain.ConfigFile
	for _, h := range sorted {
		if file, err = s.deps.Settings.AddUserInteractionTool(ctx, level, h, tool); err != nil {
			return SettingsEditResult{}, err
		}
	}
	report := sess.Reanalyze(updated)
	report.Quality = r.Quality
	return SettingsEditResult{
		Report:      report,
		ConfigPath:  file.Path,
		ConfigLevel: level,
		Tool:        tool,
		Harnesses:   sorted,
	}, nil
}

func sortedUniqueHarnesses(in []domain.Harness) []domain.Harness {
	seen := make(map[domain.Harness]bool, len(in))
	var out []domain.Harness
	for _, h := range in {
		if !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
