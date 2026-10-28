package cli

import (
	"encoding/json"
	"io"

	"mosaic-log-analyzer/internal/app"
	"mosaic-log-analyzer/internal/domain"
)

// ---------------------------------------------------------------------------
// Token encoding helpers
// ---------------------------------------------------------------------------

// jsonTokenUsage is the JSON wire shape for a token usage block.
// Absent categories encode as JSON null; present-zero encodes as 0.
type jsonTokenUsage struct {
	Input         *int64 `json:"input"`
	CacheRead     *int64 `json:"cache_read"`
	CacheCreation *int64 `json:"cache_creation"`
	Output        *int64 `json:"output"`
}

func encodeTokenCount(tc domain.TokenCount) *int64 {
	v, ok := tc.Value()
	if !ok {
		return nil
	}
	return &v
}

func encodeTokenUsage(u domain.TokenUsage) jsonTokenUsage {
	return jsonTokenUsage{
		Input:         encodeTokenCount(u.Input),
		CacheRead:     encodeTokenCount(u.CacheRead),
		CacheCreation: encodeTokenCount(u.CacheCreation),
		Output:        encodeTokenCount(u.Output),
	}
}

// jsonReportTokenUsage is the token usage block of the report encoding: the
// total-contract block plus the reasoning part of output. Reasoning is display
// only; it is null when not provided.
type jsonReportTokenUsage struct {
	jsonTokenUsage
	Reasoning *int64 `json:"reasoning"`
}

func encodeReportTokenUsage(u domain.TokenUsage) jsonReportTokenUsage {
	return jsonReportTokenUsage{
		jsonTokenUsage: encodeTokenUsage(u),
		Reasoning:      encodeTokenCount(u.Reasoning),
	}
}

// ---------------------------------------------------------------------------
// Money encoding helpers
// ---------------------------------------------------------------------------

// jsonMoneyValue is the JSON wire shape for a money cell.
// state is "known", "unpriced", or "no_data".
// amount is present only when state == "known"; it is an exact decimal USD
// string with trailing zeros trimmed, never a JSON number.
type jsonMoneyValue struct {
	State  string  `json:"state"`
	Amount *string `json:"amount,omitempty"`
}

func encodeMoneyValue(mv domain.MoneyValue) jsonMoneyValue {
	switch mv.State {
	case domain.MoneyKnown:
		s := mv.Amount.String()
		return jsonMoneyValue{State: "known", Amount: &s}
	case domain.MoneyUnpriced:
		return jsonMoneyValue{State: "unpriced"}
	default: // MoneyNoData
		return jsonMoneyValue{State: "no_data"}
	}
}

// ---------------------------------------------------------------------------
// EncodeRunTotal — stable basic total-per-run contract (FR-19)
// ---------------------------------------------------------------------------

// jsonRunTotalWire is the stable JSON wire shape for the basic total-per-run
// contract. Its field set is additive-only; no field is ever removed or retyped.
//
// Wire shape (schema_version "1"):
//
//	{
//	  "schema_version": "1",
//	  "run_id":         "20260101T000000Z-abcd",
//	  "provisional":    false,
//	  "currency":       "USD",
//	  "tokens":         { "input": 123, "cache_read": null, "cache_creation": 0, "output": 7 },
//	  "money":          { "state": "known", "amount": "1.23" },
//	  "complete":       true
//	}
type jsonRunTotalWire struct {
	SchemaVersion  string          `json:"schema_version"`
	RunID          string          `json:"run_id"`
	Provisional    bool            `json:"provisional"`
	Currency       string          `json:"currency"`
	Tokens         jsonTokenUsage  `json:"tokens"`
	Money          jsonMoneyValue  `json:"money"`
	Complete       bool            `json:"complete"`
	UnpricedModels []string        `json:"unpriced_models,omitempty"`
	PartialMoney   *jsonMoneyValue `json:"partial_money,omitempty"`

	// UnknownRunMerged is the count of usage records merged from the
	// unknown-run sibling bucket. Absent (omitempty) when zero.
	UnknownRunMerged int `json:"unknown_run_merged,omitempty"`

	// UnknownRunResidual is the count of unknown-run records that could
	// not be attributed after merging. Absent (omitempty) when zero.
	UnknownRunResidual int `json:"unknown_run_residual,omitempty"`

	// UsageNotFullyIngested is true when usage of the run is pending in its
	// ingest state or that state is unreadable. Absent otherwise.
	UsageNotFullyIngested bool `json:"usage_not_fully_ingested,omitempty"`

	// PendingUsageRecords is the number of pending usage spans. Absent when zero.
	PendingUsageRecords int `json:"pending_usage_records,omitempty"`
}

// EncodeRunTotal writes the stable basic total-per-run contract to w.
// The field set is additive-only; no field is ever removed or retyped.
func EncodeRunTotal(w io.Writer, t app.RunTotal) error {
	// Listing choice for `total --format json`: unpriced_models is a list of
	// model-id strings (the AgentTest contract), and a model id that is
	// unpriced under several providers is listed once. RunTotal already
	// carries distinct model ids, so they are copied as they are.
	var unpricedModels []string
	for _, m := range t.UnpricedModels {
		unpricedModels = append(unpricedModels, string(m))
	}

	var partialMoney *jsonMoneyValue
	if t.PartialAmount.State == domain.MoneyKnown {
		enc := encodeMoneyValue(t.PartialAmount)
		partialMoney = &enc
	}

	wire := jsonRunTotalWire{
		SchemaVersion:      "1",
		RunID:              t.RunID,
		Provisional:        t.Provisional,
		Currency:           domain.Currency,
		Tokens:             encodeTokenUsage(t.Tokens),
		Money:              encodeMoneyValue(t.Money),
		Complete:           t.Complete,
		UnpricedModels:     unpricedModels,
		PartialMoney:       partialMoney,
		UnknownRunMerged:   t.UnknownRunMerged,
		UnknownRunResidual: t.UnknownRunResidual,

		UsageNotFullyIngested: t.PendingUsageRecords > 0 || t.UsageStateUnreadable,
		PendingUsageRecords:   t.PendingUsageRecords,
	}
	return json.NewEncoder(w).Encode(wire)
}

// ---------------------------------------------------------------------------
// EncodeReport — full report JSON
// ---------------------------------------------------------------------------

// jsonCategoryMoney is the per-category money breakdown used in totals.
type jsonCategoryMoney struct {
	Input         jsonMoneyValue `json:"input"`
	CacheRead     jsonMoneyValue `json:"cache_read"`
	CacheCreation jsonMoneyValue `json:"cache_creation"`
	Output        jsonMoneyValue `json:"output"`
	Total         jsonMoneyValue `json:"total"`
	Complete      bool           `json:"complete"`
}

func encodeCategoryMoney(cm domain.CategoryMoney) jsonCategoryMoney {
	return jsonCategoryMoney{
		Input:         encodeMoneyValue(cm.Input),
		CacheRead:     encodeMoneyValue(cm.CacheRead),
		CacheCreation: encodeMoneyValue(cm.CacheCreation),
		Output:        encodeMoneyValue(cm.Output),
		Total:         encodeMoneyValue(cm.Total),
		Complete:      cm.Complete,
	}
}

// jsonTotals pairs tokens and money side by side.
type jsonTotals struct {
	Tokens jsonReportTokenUsage `json:"tokens"`
	Money  jsonCategoryMoney    `json:"money"`
	// AssumedTierCacheWrite counts cache-write tokens priced at the assumed 5m
	// tier; absent when zero.
	AssumedTierCacheWrite int64 `json:"assumed_tier_cache_write_tokens,omitempty"`
}

func encodeTotals(t domain.Totals) jsonTotals {
	return jsonTotals{
		Tokens:                encodeReportTokenUsage(t.Tokens),
		Money:                 encodeCategoryMoney(t.Money),
		AssumedTierCacheWrite: t.AssumedTierCacheWrite,
	}
}

// jsonActorReport is the per-actor breakdown within a run.
type jsonActorReport struct {
	Actor     string   `json:"actor"`
	AgentType string   `json:"agent_type,omitempty"`
	Models    []string `json:"models"`
	// ModelKeys lists each distinct (provider, model) pair the actor's
	// requests used; provider is empty when the request logged none.
	ModelKeys []jsonModelKey `json:"model_keys"`
	// RequestsWithoutModel counts token-bearing requests that logged no model.
	RequestsWithoutModel int          `json:"requests_without_model"`
	UnpricedModels       []string     `json:"unpriced_models"`
	Totals               jsonTotals   `json:"totals"`
	Effort               []jsonEffort `json:"effort"`

	// Context is set for the orchestrator and agents of a run report only.
	Context *jsonActorContext `json:"context,omitempty"`
}

// jsonModelKey is one provider/model pair as logged.
type jsonModelKey struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func encodeModelKeys(keys []domain.PricingKey) []jsonModelKey {
	out := make([]jsonModelKey, len(keys))
	for i, k := range keys {
		out[i] = jsonModelKey{Provider: string(k.Provider), Model: string(k.Model)}
	}
	return out
}

func encodeActorReport(a domain.ActorReport) jsonActorReport {
	models := make([]string, len(a.Models))
	for i, m := range a.Models {
		models[i] = string(m)
	}
	unpriced := encodeUnpricedModels(a.UnpricedModels)
	return jsonActorReport{
		Actor:                a.Actor.Label(),
		AgentType:            a.Actor.Type,
		Models:               models,
		ModelKeys:            encodeModelKeys(a.Keys),
		RequestsWithoutModel: a.RequestsWithoutModel,
		UnpricedModels:       unpriced,
		Totals:               encodeTotals(a.Totals),
		Effort:               encodeEffort(a.Effort),
	}
}

// jsonRunReport is one run's priced breakdown.
type jsonRunReport struct {
	RunID          string            `json:"run_id"`
	Provisional    bool              `json:"provisional"`
	Totals         jsonTotals        `json:"totals"`
	Orchestrator   jsonActorReport   `json:"orchestrator"`
	Agents         []jsonActorReport `json:"agents"`
	UnpricedModels []string          `json:"unpriced_models"`

	Overview    jsonOverview         `json:"overview"`
	ByAgentType []jsonAgentTypeGroup `json:"by_agent_type"`
	ByStatus    jsonStatusBreakdown  `json:"by_status"`
	Tools       jsonToolsBreakdown   `json:"tools"`
	Timeline    jsonTimeline         `json:"timeline"`

	OutcomeSplit jsonOutcomeSplit `json:"outcome_split"`
	Rework       jsonRework       `json:"rework"`
	Cache        jsonCache        `json:"cache"`
	WallTime     jsonWallTime     `json:"wall_time"`

	// Unattributed is the run-level usage of no orchestrator or agent; absent when the run has none.
	Unattributed *jsonActorReport `json:"unattributed,omitempty"`

	// UsageIngest is present only when usage of the run is still pending or its
	// ingest state is unreadable.
	UsageIngest *jsonUsageIngest `json:"usage_ingest,omitempty"`
}

// jsonUsageIngest is the wire shape of a run's pending-usage state.
type jsonUsageIngest struct {
	PendingRecords  int  `json:"pending_records,omitempty"`
	StateUnreadable bool `json:"state_unreadable,omitempty"`
}

func encodeRunReport(r domain.RunReport) jsonRunReport {
	agents := make([]jsonActorReport, 0, len(r.Agents))
	for _, a := range r.Agents {
		enc := encodeActorReport(a)
		enc.Context = encodeRunActorContext(r.Context, a)
		agents = append(agents, enc)
	}
	orchestrator := encodeActorReport(r.Orchestrator)
	orchestrator.Context = encodeRunActorContext(r.Context, r.Orchestrator)
	unpriced := encodeUnpricedModels(r.UnpricedModels)
	out := jsonRunReport{
		RunID:          r.Run.Label(),
		Provisional:    r.Provisional,
		Totals:         encodeTotals(r.Totals),
		Orchestrator:   orchestrator,
		Agents:         agents,
		UnpricedModels: unpriced,

		Overview:    encodeOverview(r.Overview),
		ByAgentType: encodeAgentTypeGroups(r.ByAgentType),
		ByStatus:    encodeStatusBreakdown(r.ByStatus),
		Tools:       encodeToolsBreakdown(r.Tools),
		Timeline:    encodeTimeline(r.Timeline),

		OutcomeSplit: encodeOutcomeSplit(r.Overview.Outcomes),
		Rework:       encodeRework(r.Rework),
		Cache:        encodeCache(r.Cache),
		WallTime:     encodeWallTime(r.WallTime),
	}
	if r.Unattributed != nil {
		u := encodeActorReport(*r.Unattributed)
		out.Unattributed = &u
	}
	if st := r.UsageIngest; st != (domain.UsageIngestState{}) {
		out.UsageIngest = &jsonUsageIngest{PendingRecords: st.PendingRecords, StateUnreadable: st.StateUnreadable}
	}
	return out
}

// jsonFinding is the JSON wire shape for a data-quality finding.
type jsonFinding struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	RunID    string `json:"run_id,omitempty"`
	Actor    string `json:"actor,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// jsonQuality is the JSON wire shape for the data-quality summary.
type jsonQuality struct {
	Incomplete bool           `json:"incomplete"`
	Counts     map[string]int `json:"counts"`
	Findings   []jsonFinding  `json:"findings"`
}

func encodeQuality(q domain.QualitySummary) jsonQuality {
	findings := make([]jsonFinding, 0, len(q.Findings))
	for _, f := range q.Findings {
		sev := "info"
		if f.Severity == domain.SeverityWarning {
			sev = "warning"
		}
		findings = append(findings, jsonFinding{
			Kind:     string(f.Kind),
			Severity: sev,
			Path:     f.Path,
			Line:     f.Line,
			RunID:    f.Run.Label(),
			Actor:    f.Actor.Label(),
			Detail:   f.Detail,
		})
	}
	counts := make(map[string]int)
	for k, v := range q.Counts {
		counts[string(k)] = v
	}
	return jsonQuality{
		Incomplete: q.Incomplete(),
		Counts:     counts,
		Findings:   findings,
	}
}

// jsonSource is the JSON wire shape for a log source descriptor.
type jsonSource struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

func encodeSource(s domain.Source) jsonSource {
	kind := "not_found"
	switch s.Kind {
	case domain.SourceLogsRoot:
		kind = "logs_root"
	case domain.SourceSingleRun:
		kind = "single_run"
	case domain.SourceUnusable:
		kind = "unusable"
	case domain.SourceDigestFile:
		kind = "digest_file"
	case domain.SourceDigestDir:
		kind = "digest_dir"
	}
	return jsonSource{Kind: kind, Path: s.Path}
}

// jsonReportWire is the full report JSON wire shape.
type jsonReportWire struct {
	SchemaVersion  string          `json:"schema_version"`
	Currency       string          `json:"currency"`
	Source         jsonSource      `json:"source"`
	AllRuns        jsonTotals      `json:"all_runs"`
	Runs           []jsonRunReport `json:"runs"`
	Unattributable *jsonRunReport  `json:"unattributable,omitempty"`
	UnpricedModels []string        `json:"unpriced_models"`
	AcrossRuns     jsonAcrossRuns  `json:"across_runs"`
	Quality        jsonQuality     `json:"quality"`
}

// EncodeReport writes the full report JSON to w, preserving absent-vs-zero
// for token categories and unpriced-vs-$0 for money cells.
//
// The output contains: schema_version, currency, source, all_runs, runs
// (per-run with per-agent-instance and per-category figures), unattributable
// (as a sibling of runs, never folded into runs), unpriced_models, and quality
// (incomplete flag, per-kind counts, individual findings).
func EncodeReport(w io.Writer, r domain.Report) error {
	runs := make([]jsonRunReport, 0, len(r.Runs))
	for _, run := range r.Runs {
		runs = append(runs, encodeRunReport(run))
	}

	var unattributable *jsonRunReport
	if r.Unattributable != nil {
		enc := encodeRunReport(*r.Unattributable)
		unattributable = &enc
	}

	unpriced := encodeUnpricedModels(r.UnpricedModels)

	wire := jsonReportWire{
		SchemaVersion:  "1",
		Currency:       r.Currency,
		Source:         encodeSource(r.Source),
		AllRuns:        encodeTotals(r.AllRuns),
		Runs:           runs,
		Unattributable: unattributable,
		UnpricedModels: unpriced,
		AcrossRuns:     encodeAcrossRuns(r.AcrossRuns),
		Quality:        encodeQuality(r.Quality),
	}
	return json.NewEncoder(w).Encode(wire)
}
