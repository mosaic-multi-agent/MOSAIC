package deploy

// retire_harness.go removes one harness from the manifest a run writes. It serves the one flow
// whose target replaces another harness's deployment in the same files.

import "mosaic-deploy/internal/domain"

// withoutHarness returns m without the entries and the section of harnessID. An empty
// harnessID returns m unchanged. The input is not modified.
func withoutHarness(m domain.Manifest, harnessID string) domain.Manifest {
	if harnessID == "" {
		return m
	}
	out := m
	out.Entries = make([]domain.ManifestEntry, 0, len(m.Entries))
	for _, e := range m.Entries {
		if e.Harness != harnessID {
			out.Entries = append(out.Entries, e)
		}
	}
	out.Harnesses = make([]domain.HarnessSection, 0, len(m.Harnesses))
	for _, s := range m.Harnesses {
		if s.HarnessID != harnessID {
			out.Harnesses = append(out.Harnesses, s)
		}
	}
	return out
}
