// Package manifesttest holds helpers for tests that build manifests of several harnesses.
package manifesttest

import "mosaic-deploy/internal/domain"

// WithHarness returns a copy of entries in which every entry without a harness belongs to
// harnessID. Entries that already name a harness are kept as they are.
func WithHarness(harnessID string, entries []domain.ManifestEntry) []domain.ManifestEntry {
	if entries == nil {
		return nil
	}
	out := make([]domain.ManifestEntry, len(entries))
	for i, e := range entries {
		if e.Harness == "" {
			e.Harness = harnessID
		}
		out[i] = e
	}
	return out
}
