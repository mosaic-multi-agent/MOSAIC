package app

// deployed_read.go is the home of the deployed-artifact read funnel.
//
// This file is the ONLY place in the application layer permitted to call a
// translator's Decode. Every site that must decode deployed agent bytes routes
// through one of the three layers here.
//
// # Layer summary
//
//   - Layer 1 (decodeDeployedBytes): bytes in hand, no filesystem, no plan item.
//   - Layer 2 (readDeployedArtifact): reads a file from disk, no plan item.
//   - Layer 3 (readDeployedPlanItem): knows about plan items, calls Layer 2.
//
// Skills and hooks pass through undecoded at every layer; only agent artifacts
// are decoded, and always through the translator named by the source harness
// descriptor's AgentFormatID.
//
// # Reporting channel
//
// DeployedRead.Report carries the translator's decode report. Every consumer of
// the funnel MUST surface it; see the Decode report surfacing table in
// ContractsDesign.md for which consumer surfaces it where. The helpers
// mergeDecodeReport and decodeReportNotices perform the translation from
// agentformat vocabulary to the run's reporting vocabulary.
//
// # Read-site inventory
//
// This block inventories every application-layer site in internal/app that reads
// deployed agent bytes. Each one routes through the funnel.
//
// FUNNEL-ROUTED READS:
//   - deployed_bytes.go (deployedBytes.funnelReader): the per-item deployed-bytes rule
//     shared by Deploy, Deploy agents, Update and Update workflows. It reads through
//     readDeployedArtifact (or decodeDeployedBytes for bytes captured before a migration)
//     and derives the operation from the plan item action, so the encoder always receives
//     the prior deployed bytes it needs for an update.
//     deployedBytes.canonicalReader wraps it for read-only consumers (annotateToolsChanges);
//     their decode reports are surfaced by the probe of the same files (logProbeDecodeNotices).
//   - deploy.go: deployedBytes.canonicalReader.
//   - deployagents_service.go: deployedBytes.canonicalReader.
//   - update.go: deployedBytes.canonicalReader.
//   - workflow_update.go: deployedBytes.canonicalReader.
//   - deployedstate.go (probeDeployedArtifact): readDeployedArtifact.
//   - deployed_agent_index.go: decodeDeployedBytes on files matching the harness-declared
//     agent extension.
//   - deployed_owned_written.go: decodeDeployedBytes on the bytes an agent write puts on disk, so
//     the manifest record is computed from the same decoded form the probe sees.
//   - deployed_decode_problem.go: readDeployedArtifact.
//   - deployed_parse_check.go: readDeployedArtifact.
//   - orchestrator_parse_check.go (unreadableAgentProblem): readDeployedArtifact.
//   - harness_only_discovery.go (scanHarnessOnlyAgents): readDeployedArtifact.
//   - workspace_agent_scan.go: decodeDeployedBytes in the scan.
//   - update_kept_workflows.go: readDeployedArtifact.
//   - promote_service.go (Promote): readDeployedArtifact.
//   - transform_source_models.go (ReadSourceModel, IndexSourceModels): decodeDeployedBytes.
//   - transform_plan_discovery.go: readDeployedArtifact.
//   - transform_run_build.go: readDeployedArtifact on the destination, so the encoder
//     receives the destination's prior bytes.
//   - rebuild.go: decodeDeployedBytes on the rebuild source and the destination.
//   - rebuild_destination.go: decodeDeployedBytes on the rebuild destination.
//   - rebuild_source.go: decodeDeployedBytes on the rebuild source.
//
// deployed_bytes_migration.go is not a funnel caller: it only captures the raw bytes of a
// recorded legacy file, which the funnelReader decodes through decodeDeployedBytes.
//
// Harness-only refresh (harness_only_refresh.go) receives canonical bytes from the funnel
// and threads DeployedRaw as the prior-bytes channel for the encode step, so user-owned
// keys survive in the harness's own format.
//
// GENERIC-SOURCE PARSES:
//   Calls to the raw docformat entry points operate on canonical bytes (already decoded
//   through the funnel) or on generic MOSAIC source files, and go through
//   ParseGenericSource in parse_generic_source.go, the second permitted caller of those
//   entry points besides this file. Examples: bundle_conformance.go, protocol_version.go,
//   promote.go, promote_service.go, harness_only_refresh.go, render_service_impl.go
//   (always a generic-form source, never a deployed native file).
//
// NOT DEPLOYED AGENT READS (outside the funnel's scope):
//   - readDeployedFile in summary.go: returns raw on-disk bytes for non-agent uses
//     (hooks and skills pass through undecoded).
//   - internal/harness/injectionfile/deployed.go: parses deployed harness injection files.
//   - internal/registry/descriptoronly.go: reads harness descriptor files.
//
// Paths come from the descriptor: the agents and skills locations are resolved
// independently through module.TargetPath, so no site derives one from the other.
//
// The FUNNEL-ROUTED READS list is enforced by the funnel-inventory rule in
// tools/importcheck/funnel_inventory.go (a non-test file referencing a funnel identifier
// must be listed, and a listed file must reference one); raw docformat calls are enforced by
// the read-boundary rule in tools/importcheck/main.go. Both run in go test ./....

import (
	"fmt"
	"os"
	"path/filepath"

	"mosaic-deploy/internal/agentformat"
	"mosaic-deploy/internal/domain"
	"mosaic-deploy/internal/transform"
)

// DeployedRead is the funnel's single result value. Raw and Canonical are returned
// together so that a caller cannot obtain one without the other and cannot accidentally
// thread only the decoded form.
//
// Outcomes:
//   - absent (a create):    Present false, ReadErr nil,  Canonical nil, DecodeErr nil, Report zero
//   - unreadable:           Present true,  ReadErr set,  Canonical nil, DecodeErr nil, Report zero
//   - corrupt (bad decode): Present true,  ReadErr nil,  Canonical nil, DecodeErr set, Report zero
//   - ok:                   Present true,  ReadErr nil,  Canonical set, DecodeErr nil, Report the translator's decode report
//
// The two error channels (ReadErr and DecodeErr) never both carry a failure.
type DeployedRead struct {
	// Raw is the bytes exactly as read from disk, in the harness's own format.
	// Nil when the file is absent or unreadable.
	Raw []byte

	// Canonical is the decoded canonical bytes (Markdown with YAML frontmatter).
	// Nil when DecodeErr is set, or when the file is absent or unreadable.
	// For Markdown harnesses, Canonical equals Raw (the identity decode).
	Canonical []byte

	// Present is false only when the file does not exist on disk.
	// It is NOT set to false for unreadable or undecodable files.
	Present bool

	// ReadErr is set when the file exists on disk but could not be read.
	// It is distinct from a missing file (Present false) and from a decode error.
	ReadErr error

	// DecodeErr is set when the file was read successfully but its bytes could not
	// be decoded. Callers classify with errors.Is(DecodeErr, agentformat.ErrMalformedDeployed).
	// Non-conflict plan items must fail the artifact on a non-nil DecodeErr;
	// conflict items resolved by overwrite must proceed (see ContractsDesign.md).
	DecodeErr error

	// Report is the DECODE report, exactly as the translator returned it.
	// It is always a value (never a pointer), and an empty Entries slice is the
	// normal case: for a Markdown harness it is always empty.
	// Set only on the ok outcome; absent, unreadable or undecodable files leave it zero.
	// Consumers MUST surface it; see "Decode report surfacing" in ContractsDesign.md.
	Report agentformat.Report
}

// decodeDeployedBytes is Layer 1 of the decode funnel.
//
// It takes raw bytes already in hand, the artifact kind, and the source harness
// descriptor. It returns a DeployedRead with both the raw bytes and the decoded
// canonical bytes (or a named decode error). No filesystem access; no plan item.
//
// This is the ONLY place in the application layer that calls a translator's Decode.
// It resolves the translator with agentformat.LookupString(src.AgentFormatID) and
// never with formatid.Parse followed by Lookup (CD-4).
//
// Skills and hooks pass through undecoded: their DeployedRead has Raw == Canonical == raw.
func decodeDeployedBytes(raw []byte, kind domain.ArtifactKind, src *domain.HarnessDescriptor) DeployedRead {
	if kind != domain.ArtifactAgent {
		// Skills and hooks are not decoded; Raw and Canonical are the same bytes.
		return DeployedRead{Raw: raw, Canonical: raw, Present: len(raw) > 0}
	}

	// Guard: a nil descriptor is treated as a zero-value descriptor (Markdown identity
	// harness). Both buildDeployedAgentIndex and scanWorkspaceAgents document that a nil
	// src is treated as Markdown; this guard enforces that at the decode boundary so
	// callers cannot panic by passing nil through to src.AgentFormatID.
	if src == nil {
		src = &domain.HarnessDescriptor{}
	}

	t, err := agentformat.LookupString(src.AgentFormatID)
	if err != nil {
		// A format ID that fails to resolve is a descriptor defect. Surface it as a
		// decode error with the format ID problem so the caller can fail the artifact.
		return DeployedRead{Raw: raw, Present: len(raw) > 0, DecodeErr: err}
	}

	canonical, report, err := t.Decode(raw, agentformat.ArtifactContext{
		Kind: kind,
	})
	if err != nil {
		return DeployedRead{Raw: raw, Present: len(raw) > 0, DecodeErr: err}
	}
	return DeployedRead{
		Raw:       raw,
		Canonical: canonical,
		Present:   true,
		Report:    report,
	}
}

// readDeployedArtifact is Layer 2 of the decode funnel.
//
// It reads a file at the given path, then delegates to decodeDeployedBytes (Layer 1).
// It adds the presence flag and a read error channel on top of Layer 1's result.
// No plan item; this is the entry point that Stages 8, 9 and 10 use for all deployed-
// agent reads that hold no plan item.
//
// Outcomes:
//   - File absent:     DeployedRead{Present: false}
//   - File unreadable: DeployedRead{Present: true, ReadErr: <error>}
//   - Decode failure:  DeployedRead{Present: true, Raw: <bytes>, DecodeErr: <error>}
//   - OK:              DeployedRead{Present: true, Raw: <bytes>, Canonical: <bytes>, Report: <report>}
func readDeployedArtifact(path string, kind domain.ArtifactKind, src *domain.HarnessDescriptor) DeployedRead {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DeployedRead{Present: false}
		}
		// File exists but could not be read: present, with a read error.
		return DeployedRead{Present: true, ReadErr: err}
	}
	// File was read; delegate decode to Layer 1.
	result := decodeDeployedBytes(data, kind, src)
	result.Present = true
	return result
}

// actionToOp maps a domain.PlanAction to the agentformat.Operation that the
// transform/agentformat layers require. It is the single definition of that mapping;
// no caller may re-derive it, and no layer below app may infer it from Deployed being
// nil or non-nil (see transform/types.go Op contract).
//
// The mapping is total over the four defined PlanAction values:
//
//	ActionCreate     ("create")           -> OpCreate
//	ActionUpdate     ("update")           -> OpUpdate
//	ActionUnchanged  ("unchanged")        -> OpUpdate
//	ActionConflict   ("locally-modified") -> OpUpdate
//
// Any other value, including the zero value PlanAction(""), returns
// (OpUnspecified, non-nil error). The error carries no subject; callers add their own
// context so their existing messages are unchanged.
func actionToOp(action domain.PlanAction) (agentformat.Operation, error) {
	switch action {
	case domain.ActionCreate:
		return agentformat.OpCreate, nil
	case domain.ActionUpdate, domain.ActionUnchanged, domain.ActionConflict:
		return agentformat.OpUpdate, nil
	default:
		return agentformat.OpUnspecified, fmt.Errorf("unrecognised PlanAction %q", action)
	}
}

// readDeployedPlanItem is Layer 3 of the decode funnel.
//
// It resolves the target path and artifact kind from the plan item, calls
// readDeployedArtifact (Layer 2), and maps PlanItem.Action to an agentformat.Operation.
// This is the only layer that knows about plan items; nothing below it may take one.
//
// The action mapping is total over the four defined PlanAction values:
//
//	ActionCreate     ("create")           -> OpCreate
//	ActionUpdate     ("update")           -> OpUpdate
//	ActionUnchanged  ("unchanged")        -> OpUpdate
//	ActionConflict   ("locally-modified") -> OpUpdate
//
// An unrecognised PlanAction value (including the zero value PlanAction("")) yields a
// non-nil error. This is the safety net against a new action being added to the domain
// without updating the funnel.
//
// The third return is plan-item-level only. It is non-nil exactly when the plan item
// cannot be turned into a read at all (an unrecognised Action). Every outcome of
// actually touching the file (absent, unreadable, undecodable, ok) lands inside
// DeployedRead and leaves this error nil. The two channels never both carry a failure.
func readDeployedPlanItem(workspace string, desc *domain.HarnessDescriptor, item domain.PlanItem) (DeployedRead, agentformat.Operation, error) {
	op, err := actionToOp(item.Action)
	if err != nil {
		return DeployedRead{}, agentformat.OpUnspecified,
			fmt.Errorf("readDeployedPlanItem: %w for item %q", err, item.TargetPath)
	}

	path := filepath.Join(workspace, item.TargetPath)
	read := readDeployedArtifact(path, item.Ref.Kind, desc)
	return read, op, nil
}

// mergeDecodeReport appends dr's entries to dst using the entry-kind mapping table
// from the transform pipeline contract. It is additive: it never clears, re-orders
// or de-duplicates what dst already holds. Calling it twice for one artifact duplicates
// the entries, so each consumer calls it exactly once, at the named point.
//
// This is the single place the decode-side vocabulary is translated into the run's
// reporting vocabulary; a consumer that formats agentformat entries itself is a second
// rendering of the same facts and is forbidden.
func mergeDecodeReport(dst *transform.Report, dr agentformat.Report) {
	for _, entry := range dr.Entries {
		switch entry.Kind {
		case agentformat.EntryMissingInstructions:
			// decode-only: a deployed file carried no developer_instructions.
			// Not a FieldChange (the agent body is not a frontmatter key).
			// No existing gap kind fits exactly; surfaced as a manual-step gap so
			// it reaches the TODO output and is visible to the user.
			dst.Gaps = append(dst.Gaps, domain.Gap{
				Kind:   domain.GapManualStep,
				Detail: entry.Detail,
			})
		case agentformat.EntryDroppedForeignKey:
			dst.Fields = append(dst.Fields, transform.FieldChange{
				Key:    entry.Key,
				Before: entry.Detail,
				After:  "",
				Reason: entry.Reason,
			})
		case agentformat.EntryOverriddenName:
			dst.Fields = append(dst.Fields, transform.FieldChange{
				Key:    entry.Key,
				Before: entry.Detail,
				Reason: entry.Reason,
			})
		case agentformat.EntryAppliedFallback:
			dst.Fields = append(dst.Fields, transform.FieldChange{
				Key:    entry.Key,
				After:  entry.Detail,
				Reason: entry.Reason,
			})
		case agentformat.EntryStrippedContainer:
			dst.Fields = append(dst.Fields, transform.FieldChange{
				Key:    entry.Key,
				Before: entry.Detail,
				After:  "",
				Reason: entry.Reason,
			})
		case agentformat.EntryCarriedContainer:
			// Nothing: the key was re-emitted, so no field changed.
		case agentformat.EntryRefusedMarker:
			dst.Fields = append(dst.Fields, transform.FieldChange{
				Key:    entry.Key,
				After:  entry.Detail,
				Reason: entry.Reason,
			})
		case agentformat.EntryDroppedComment:
			// A free-standing comment that did not travel. Not a FieldChange.
			// No existing gap kind fits; leave for a later reporting stage.
		}
	}
}

// decodeReportNotices renders dr for a consumer that has no transform.Report to merge
// into. One notice string per entry, each naming the artifact key and the entry key.
// Same vocabulary as mergeDecodeReport, different sink.
func decodeReportNotices(agentKey string, dr agentformat.Report) []string {
	var notices []string
	for _, entry := range dr.Entries {
		switch entry.Kind {
		case agentformat.EntryMissingInstructions:
			notices = append(notices, fmt.Sprintf(
				"agent %q: deployed file carries no developer_instructions (body is empty)",
				agentKey,
			))
		case agentformat.EntryDroppedForeignKey:
			notices = append(notices, fmt.Sprintf(
				"agent %q: key %q was dropped during decode (%s)",
				agentKey, entry.Key, entry.Reason,
			))
		case agentformat.EntryCarriedContainer:
			// Nothing: the key was re-emitted.
		case agentformat.EntryDroppedComment:
			notices = append(notices, fmt.Sprintf(
				"agent %q: comment was dropped during decode: %s",
				agentKey, entry.Detail,
			))
		default:
			notices = append(notices, fmt.Sprintf(
				"agent %q: decode notice [%s] key=%q: %s",
				agentKey, entry.Kind, entry.Key, entry.Reason,
			))
		}
	}
	return notices
}
