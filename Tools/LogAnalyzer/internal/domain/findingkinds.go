package domain

import "sort"

// FindingKindSummary is one overlay row and one entry of the export counts: the
// findings of a single kind.
type FindingKindSummary struct {
	Kind     FindingKind
	Severity FindingSeverity // highest severity among the kind's findings
	Count    int             // number of findings of the kind
	Runs     int             // distinct named runs (Kind RunNamed, ID != "") among them
}

// Label is "info" or "warning" (the wire values of report --format json and digests).
func (s FindingSeverity) Label() string {
	if s == SeverityWarning {
		return "warning"
	}
	return "info"
}

// kindExplanations holds the plain-language text of every kind: what it means
// and what it does to the figures.
var kindExplanations = map[FindingKind]string{
	FindingMalformedLine:              "a log line is not valid JSON; it is skipped, so its event is missing from the figures",
	FindingTruncatedLine:              "the last line of a log file was cut off mid-write; it is skipped, so its event is missing from the figures",
	FindingUnreadableFile:             "a log file could not be read; none of its events are in the figures",
	FindingUnknownSchema:              "a log line uses a schema version this tool does not know; it is skipped, so its event is missing from the figures",
	FindingUnrecognisedHarness:        "an event comes from a harness this tool does not recognise; it is skipped, so its event is missing from the figures",
	FindingMissingTokenUsage:          "a model call has no token usage; it counts as zero tokens and its cost is incomplete",
	FindingMissingModel:               "a model call names no model; its cost cannot be priced and is left out of the totals",
	FindingUnidentifiedEvent:          "an event could not be tied to an agent or dispatch; it is counted in the totals but not attributed",
	FindingUnrecognisedFolder:         "a folder in the log directory is not a run or actor folder; it is ignored",
	FindingMissingEventFile:           "a run or actor folder has no event file; nothing is read from it, so its figures are missing",
	FindingInvalidPricingEntry:        "a pricing entry is invalid; it is ignored, so the models it covers may be unpriced",
	FindingInvalidSettingsEntry:       "a settings entry is invalid; it is ignored and the default applies",
	FindingCrossStreamRecord:          "the same usage record appeared in two actors' logs; counted once, for the first actor read",
	FindingInvalidCacheTierSplit:      "cache-write tokens split across tiers do not add up; the figures use the assumed tier for them",
	FindingCompactionMismatch:         "a context compaction does not match the surrounding token counts; the context growth figures may be off",
	FindingInvalidDigest:              "a digest file is not valid; it is ignored, so its runs are missing from the report",
	FindingUnsupportedDigestVersion:   "a digest file has a version this tool does not support; it is ignored, so its runs are missing",
	FindingDuplicateRun:               "the same run was found more than once (logs and digest); it is counted once",
	FindingTranscriptFormatChanged:    "the transcript format differs from what this tool expects; transcript-based figures may be incomplete",
	FindingPricingCurrencyMismatch:    "a pricing entry uses a different currency than the rest; its amounts are not converted, so totals may mix currencies",
	FindingInstanceTypeMismatch:       "an agent instance reports different agent types in different places; the figures use the first one seen",
	FindingUnresolvedInvocation:       "an invocation could not be tied to a dispatch; it is counted but not attributed to one",
	FindingOverlappingInvocationStart: "an invocation started while another of the same agent was still open; the overlap may blur per-invocation figures",
	FindingDispatchWithoutInvocation:  "a dispatch has no matching invocation; it counts as a dispatch with no recorded work",
	FindingTranscriptCaptureGap:       "part of an agent's transcript was not captured; its usage and cost may be incomplete",
	FindingInstancePromptMismatch:     "an agent instance received a prompt that differs from its dispatch; the figures are kept but may be mis-attributed",
}

// KnownFindingKinds lists every FindingKind constant declared in package domain,
// sorted by name.
func KnownFindingKinds() []FindingKind {
	kinds := make([]FindingKind, 0, len(kindExplanations))
	for k := range kindExplanations {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	return kinds
}

// Explanation is the one-line plain-language text of the kind: what it means and
// what it does to the figures. A kind with no entry returns string(k).
func (k FindingKind) Explanation() string {
	if text, ok := kindExplanations[k]; ok {
		return text
	}
	return string(k)
}

// SummarizeFindingKinds groups findings into one FindingKindSummary per kind,
// sorted by Severity descending (warning before info), then Count descending,
// then Kind ascending. Empty input yields nil.
func SummarizeFindingKinds(findings []Finding) []FindingKindSummary {
	if len(findings) == 0 {
		return nil
	}
	index := map[FindingKind]int{}
	runs := map[FindingKind]map[string]bool{}
	var rows []FindingKindSummary
	for _, f := range findings {
		i, ok := index[f.Kind]
		if !ok {
			i = len(rows)
			index[f.Kind] = i
			rows = append(rows, FindingKindSummary{Kind: f.Kind})
			runs[f.Kind] = map[string]bool{}
		}
		rows[i].Count++
		rows[i].Severity = max(rows[i].Severity, f.Severity)
		if f.Run.Kind == RunNamed && f.Run.ID != "" {
			runs[f.Kind][f.Run.ID] = true
		}
	}
	for i := range rows {
		rows[i].Runs = len(runs[rows[i].Kind])
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Severity != b.Severity {
			return a.Severity > b.Severity
		}
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Kind < b.Kind
	})
	return rows
}
