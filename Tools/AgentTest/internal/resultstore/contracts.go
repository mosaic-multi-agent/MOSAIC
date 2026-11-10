// Package resultstore files validated AgentTest report JSON files into the
// OrchestrationTestResults/ directory tree, organised by subject version.
//
// Public types and interfaces are defined in resultstore.go.
// This file holds contract constants added to the package's public surface.
package resultstore

// SkipUnverifiedModel is the reason recorded for a report that Store refuses
// because at least one of its runs has no observed subject model or has one
// that differs from the selected subject model. The refusal is counted in
// StoreResult.SkippedUnverifiedModel and rendered in the summary line as
// "<N> refused".
const SkipUnverifiedModel SkipReason = "unverified_model"
