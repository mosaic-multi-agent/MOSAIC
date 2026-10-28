package cli

import "mosaic-log-analyzer/internal/domain"

// This file encodes the per-actor context and compaction figures of the report
// JSON. An absent context figure is JSON null, never 0.

// jsonContextFigure is one context size with its band.
type jsonContextFigure struct {
	Tokens  int64  `json:"tokens"`
	Band    string `json:"band"`
	Partial bool   `json:"partial"`
}

// jsonFirstWrite is the first-write context figure; it is a heuristic.
type jsonFirstWrite struct {
	jsonContextFigure
	Heuristic bool `json:"heuristic"`
}

type jsonCompactionCounts struct {
	Logged   int `json:"logged"`
	Detected int `json:"detected"`
}

// jsonDetectedCompaction is a compaction inferred from a context drop.
type jsonDetectedCompaction struct {
	Label        string  `json:"label"`
	FromAt       *string `json:"from_at"`
	ToAt         *string `json:"to_at"`
	BeforeTokens int64   `json:"before_tokens"`
	AfterTokens  int64   `json:"after_tokens"`
}

type jsonActorContext struct {
	Calls               int                      `json:"calls"`
	First               *jsonContextFigure       `json:"first"`
	Peak                *jsonContextFigure       `json:"peak"`
	Final               *jsonContextFigure       `json:"final"`
	FinalDefinition     string                   `json:"final_definition"`
	FirstWrite          *jsonFirstWrite          `json:"first_write"`
	Compactions         jsonCompactionCounts     `json:"compactions"`
	DetectedCompactions []jsonDetectedCompaction `json:"detected_compactions"`
}

// encodeContextFigure returns nil for an absent figure.
func encodeContextFigure(f domain.ContextFigure) *jsonContextFigure {
	tokens, ok := f.Tokens.Value()
	if !ok {
		return nil
	}
	return &jsonContextFigure{Tokens: tokens, Band: f.Band.Key(), Partial: f.Partial}
}

func encodeFirstWrite(f domain.ContextFigure) *jsonFirstWrite {
	fig := encodeContextFigure(f)
	if fig == nil {
		return nil
	}
	return &jsonFirstWrite{jsonContextFigure: *fig, Heuristic: true}
}

func encodeActorContext(a domain.ActorContext) jsonActorContext {
	detected := []jsonDetectedCompaction{}
	for _, m := range a.Compactions {
		if m.Kind != domain.CompactionDetected {
			continue
		}
		detected = append(detected, jsonDetectedCompaction{
			Label:        "detected",
			FromAt:       callTimeOrNull(a.Calls, m.Before),
			ToAt:         callTimeOrNull(a.Calls, m.After),
			BeforeTokens: m.BeforeTokens,
			AfterTokens:  m.AfterTokens,
		})
	}
	return jsonActorContext{
		Calls:               len(a.Calls),
		First:               encodeContextFigure(a.First),
		Peak:                encodeContextFigure(a.Peak),
		Final:               encodeContextFigure(a.Final),
		FinalDefinition:     domain.FinalContextDefinition,
		FirstWrite:          encodeFirstWrite(a.FirstWrite),
		Compactions:         jsonCompactionCounts{Logged: a.LoggedCount(), Detected: a.DetectedCount()},
		DetectedCompactions: detected,
	}
}

// callTimeOrNull returns the time of the indexed call, or null for an index
// outside the calls.
func callTimeOrNull(calls []domain.ContextCall, i int) *string {
	if i < 0 || i >= len(calls) {
		return nil
	}
	return timeOrNull(calls[i].At)
}

// encodeRunActorContext returns the context of the actor matching report, or
// an empty context when the run holds none for it.
func encodeRunActorContext(run domain.RunContext, a domain.ActorReport) *jsonActorContext {
	ctx := run.Orchestrator
	if a.Actor.Kind == domain.ActorAgentInstance {
		ctx, _ = run.Find(a.Actor.Instance)
	}
	enc := encodeActorContext(ctx)
	return &enc
}
