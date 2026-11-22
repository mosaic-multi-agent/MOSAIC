package analysis

import "mosaic-log-analyzer/internal/domain"

// OverlappingStartFindings scans every stream with Ref.Kind == StreamAgentInstance
// in event order. An invocation_end closes the stream's open start; an
// invocation_start arriving while a start is open gives one
// overlapping_invocation_start finding (warning severity, Run = Ref.Run,
// Actor = the folder id Ref.InstanceHint, Path = Ref.Path).
func OverlappingStartFindings(streams []Stream) []domain.Finding {
	var findings []domain.Finding
	for _, s := range streams {
		if s.Ref.Kind != domain.StreamAgentInstance {
			continue
		}
		open := false
		for _, ev := range s.Events {
			switch ev.Type {
			case domain.EventInvocationEnd:
				open = false
			case domain.EventInvocationStart:
				if open {
					findings = append(findings, domain.Finding{
						Kind: domain.FindingOverlappingInvocationStart, Severity: domain.SeverityWarning,
						Path: s.Ref.Path, Run: s.Ref.Run, Actor: domain.AgentInstance(s.Ref.InstanceHint),
						Detail: "invocation_start arrived before the previous invocation_start had an invocation_end",
					})
				}
				open = true
			}
		}
	}
	return findings
}
