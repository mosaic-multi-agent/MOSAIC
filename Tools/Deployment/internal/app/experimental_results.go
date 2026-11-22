package app

// experimental_results.go supplies the experimental-harness warning carried by operation results.

import "mosaic-deploy/internal/domain"

// experimentalOfRun lists the experimental harnesses a transform run has resolved so far,
// source first. A harness that is not resolved yet (nil) is not reported.
func (r *transformRun) experimentalOfRun() []domain.ExperimentalHarness {
	var refs []domain.HarnessRef
	if r.src != nil {
		refs = append(refs, r.src.Ref())
	}
	if r.tgt != nil {
		refs = append(refs, r.tgt.Ref())
	}
	return domain.ExperimentalHarnessesOf(refs...)
}
