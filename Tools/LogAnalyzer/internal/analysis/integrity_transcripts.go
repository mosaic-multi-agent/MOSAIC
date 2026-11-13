package analysis

import (
	"fmt"
	"strings"

	"mosaic-log-analyzer/internal/domain"
)

// positionHashPrefix marks a usage record id derived from the record's position
// rather than from a transcript message id; such ids are never compared.
const positionHashPrefix = "pos:"

// streamKey identifies the stream a transcript belongs to.
type streamKey struct {
	run   domain.RunRef
	owner domain.StreamKind
	hint  domain.AgentInstanceID
}

// streamUsage is what the capture gap check needs from one stream.
type streamUsage struct {
	starts int
	ids    map[string]bool // comparable usage record ids (no empty, no position hash)
}

// TranscriptCaptureGapFindings compares each claude-code transcript's assistant
// ids with the usage records of the stream the transcript belongs to (matched by
// run, owner and folder id; the stream's own events, not the aggregate). Per
// transcript it gives one transcript_capture_gap warning for the transcript ids
// without a usage record (side a) and one for the usage record ids missing from
// the transcript (side b), each stating its count. Actor is the folder id (the
// orchestrator for the orchestrator transcript).
//
// Agent streams are compared only when they hold exactly one invocation_start: a
// resume overwrites the transcript, so several starts cannot be compared. The
// orchestrator is compared for side (a) only, because its shared transcript may
// lag the events or hold another session. Other formats and facts without a
// matching stream are skipped.
func TranscriptCaptureGapFindings(streams []Stream, facts []domain.TranscriptFacts) []domain.Finding {
	usage := collectStreamUsage(streams)
	var findings []domain.Finding
	for _, f := range facts {
		if f.Format != domain.TranscriptClaudeCodeJSONL {
			continue
		}
		su, ok := usage[streamKey{run: f.Run, owner: f.Owner, hint: f.InstanceHint}]
		if !ok {
			continue
		}
		isOrch := f.Owner == domain.StreamOrchestrator
		if !isOrch && su.starts != 1 {
			continue
		}
		actor := domain.Orchestrator()
		if !isOrch {
			actor = domain.AgentInstance(f.InstanceHint)
		}
		inTranscript := make(map[string]bool, len(f.AssistantIDs))
		for _, id := range f.AssistantIDs {
			inTranscript[id] = true
		}
		noUsage := 0
		for id := range inTranscript {
			if !su.ids[id] {
				noUsage++
			}
		}
		if noUsage > 0 {
			findings = append(findings, captureGap(f, actor, fmt.Sprintf(
				"%d transcript assistant messages have no usage record in the log; their usage was not captured", noUsage)))
		}
		if isOrch {
			continue
		}
		notInTranscript := 0
		for id := range su.ids {
			if !inTranscript[id] {
				notInTranscript++
			}
		}
		if notInTranscript > 0 {
			findings = append(findings, captureGap(f, actor, fmt.Sprintf(
				"%d usage records in the log are not in the agent's own transcript; they may belong to another agent", notInTranscript)))
		}
	}
	return findings
}

func captureGap(f domain.TranscriptFacts, actor domain.ActorRef, detail string) domain.Finding {
	return domain.Finding{
		Kind: domain.FindingTranscriptCaptureGap, Severity: domain.SeverityWarning,
		Run: f.Run, Actor: actor, Path: f.Path, Detail: detail,
	}
}

// collectStreamUsage gathers per stream key the invocation_start count and the
// comparable usage record ids.
func collectStreamUsage(streams []Stream) map[streamKey]*streamUsage {
	out := make(map[streamKey]*streamUsage)
	for _, s := range streams {
		if s.Ref.Run.IsUnattributable() {
			continue
		}
		key := streamKey{run: s.Ref.Run, owner: s.Ref.Kind, hint: s.Ref.InstanceHint}
		su := out[key]
		if su == nil {
			su = &streamUsage{ids: map[string]bool{}}
			out[key] = su
		}
		for _, ev := range s.Events {
			switch ev.Type {
			case domain.EventInvocationStart:
				su.starts++
			case domain.EventUsageRecord:
				if ev.UsageRecord == nil {
					continue
				}
				id := ev.UsageRecord.RecordID
				if id != "" && !strings.HasPrefix(id, positionHashPrefix) {
					su.ids[id] = true
				}
			}
		}
	}
	return out
}

// InstancePromptMismatchFindings reports agents whose own first prompt names (in
// the structured agent_instance_id form) a different instance id than their
// folder: one instance_prompt_mismatch warning per invocation, in input order,
// with the folder id as actor. Only claude-code and opencode agent transcripts
// with a first prompt are compared; a prompt without an id is skipped.
func InstancePromptMismatchFindings(facts []domain.TranscriptFacts) []domain.Finding {
	var findings []domain.Finding
	for _, f := range facts {
		if f.Owner != domain.StreamAgentInstance || !f.HasFirstPrompt {
			continue
		}
		if f.Format != domain.TranscriptClaudeCodeJSONL && f.Format != domain.TranscriptOpenCodeMessages {
			continue
		}
		id, ok := domain.ExtractInstanceID(f.FirstPrompt)
		if !ok || id == f.InstanceHint {
			continue
		}
		findings = append(findings, domain.Finding{
			Kind: domain.FindingInstancePromptMismatch, Severity: domain.SeverityWarning,
			Run: f.Run, Actor: domain.AgentInstance(f.InstanceHint), Path: f.Path,
			Detail: fmt.Sprintf("the agent's first prompt names %s, but its folder is %s", id, f.InstanceHint),
		})
	}
	return findings
}
