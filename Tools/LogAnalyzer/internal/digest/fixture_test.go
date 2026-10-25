package digest_test

import (
	"time"

	"mosaic-log-analyzer/internal/domain"
)

const (
	runA = "20261023T191332Z-2b6a"
	runB = "20261024T080000Z-00ff"
)

// at returns a UTC base time plus sec seconds, with nanosecond precision.
func at(sec int) time.Time {
	return time.Date(2026, 10, 23, 19, 13, 32, 123456789, time.UTC).Add(time.Duration(sec) * time.Second)
}

// fullTokens has every token field present.
func fullTokens() domain.TokenUsage {
	return domain.TokenUsage{
		Input:           domain.Tokens(100),
		CacheRead:       domain.Tokens(2000),
		CacheCreation:   domain.Tokens(300),
		Output:          domain.Tokens(50),
		CacheCreation5m: domain.Tokens(200),
		CacheCreation1h: domain.Tokens(100),
		Reasoning:       domain.Tokens(20),
	}
}

// zeroTokens has every token field present with value zero.
func zeroTokens() domain.TokenUsage {
	return domain.TokenUsage{
		Input: domain.Tokens(0), CacheRead: domain.Tokens(0), CacheCreation: domain.Tokens(0),
		Output: domain.Tokens(0), CacheCreation5m: domain.Tokens(0), CacheCreation1h: domain.Tokens(0),
		Reasoning: domain.Tokens(0),
	}
}

// perRecordUsage returns usage entries covering full, partial, present-zero and
// missing-usage (no counts at all) per-record entries.
func perRecordUsage() []domain.InvocationUsage {
	return []domain.InvocationUsage{
		{
			Model: "claude-opus-4", Provider: "anthropic", Effort: "high", Harness: domain.HarnessClaudeCode,
			Usage: fullTokens(), RecordID: "rec-1", At: at(10), ServiceTier: "standard",
		},
		{
			Model: "claude-opus-4", Harness: domain.HarnessClaudeCode,
			Usage:    domain.TokenUsage{Input: domain.Tokens(7), Output: domain.Tokens(3), Reasoning: domain.Tokens(0)},
			RecordID: "rec-2", At: at(11),
		},
		{Model: "gpt-x", Provider: "openai", Harness: "x-unknown", Usage: zeroTokens(), RecordID: "rec-3", At: at(12)},
		{Harness: domain.HarnessOpenCode, RecordID: "rec-4", At: at(13)}, // missing-usage marker
	}
}

// legacyUsage returns entries from actors without usage_record: no id, tier or effort.
func legacyUsage() []domain.InvocationUsage {
	return []domain.InvocationUsage{
		{
			Model: "claude-sonnet-4", Harness: domain.HarnessClaudeCode, At: at(20),
			Usage: domain.TokenUsage{Input: domain.Tokens(11), CacheRead: domain.Tokens(0), Output: domain.Tokens(5)},
		},
		{Model: "claude-sonnet-4", Harness: domain.HarnessClaudeCode}, // legacy, no tokens, no time
	}
}

// toolCallsAllPairings returns calls in a deliberately non-alphabetical recorded order.
func toolCallsAllPairings() []domain.ToolCall {
	return []domain.ToolCall{
		{CallID: "z-call", ToolName: "Bash", Status: domain.ToolCallSuccess, StartedAt: at(30), EndedAt: at(31), Pairing: domain.ToolCallPaired},
		{CallID: "a-call", ToolName: "Read", StartedAt: at(32), Pairing: domain.ToolCallStartOnly},
		{CallID: "m-call", Status: domain.ToolCallError, EndedAt: at(33), Pairing: domain.ToolCallEndOnly},
	}
}

// richRun is a named run exercising every fact the digest carries.
func richRun() domain.RunAggregate {
	coderUsage := perRecordUsage()
	coder := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"}
	reviewer := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "reviewer#2", Type: "reviewer"}
	return domain.RunAggregate{
		Run:         domain.NamedRun(runA),
		Provisional: false,
		Orchestrator: domain.ActorAggregate{
			Actor:       domain.Orchestrator(),
			Invocations: append(perRecordUsage(), legacyUsage()...),
		},
		Agents: []domain.ActorAggregate{
			{Actor: coder, Invocations: coderUsage},
			{Actor: domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "planner#1", Type: "planner"}, Invocations: legacyUsage()},
		},
		Activity: domain.RunActivity{
			OrchestratorToolCalls: toolCallsAllPairings(),
			Invocations: []domain.Invocation{
				{
					Actor: coder, State: domain.InvocationClosed, StatusCode: "completed", Model: "claude-opus-4",
					Provider: "anthropic", StartedAt: at(5), EndedAt: at(40), EndEvents: 2,
					ToolCalls: toolCallsAllPairings(), Usage: coderUsage,
				},
				{Actor: reviewer, State: domain.InvocationOpen, StartedAt: at(41), EndEvents: 0},
			},
			Compactions: []domain.Compaction{
				{Actor: domain.Orchestrator(), At: at(60), Trigger: "auto", TokensBefore: domain.Tokens(1000), TokensAfter: domain.Tokens(200)},
				{Actor: coder, At: at(50), Trigger: "manual", TokensBefore: domain.Tokens(0)},
				{Actor: domain.Orchestrator()},
			},
		},
		Facts:         richFacts(),
		Sessions:      richSessions(),
		Turns:         richTurns(coder),
		Notifications: richNotifications(coder),
	}
}

func richFacts() domain.RunFacts {
	return domain.RunFacts{
		Harnesses:            []domain.Harness{domain.HarnessClaudeCode, "x-unknown"},
		SchemaVersions:       []string{"1.3.0", "9.9.9"},
		AdapterVersion:       "0.7.1",
		OrchestratorModel:    "claude-opus-4",
		OrchestratorProvider: "anthropic",
		StartedAt:            at(0),
		EndedAt:              at(100),
		Outcome:              "success",
	}
}

func richSessions() []domain.RunSession {
	return []domain.RunSession{
		{
			ID: "sess-1", StartedAt: at(1), EndedAt: at(90), Resumed: domain.BoolTrue,
			Reason: "clear", Model: "claude-opus-4", Provider: "anthropic",
		},
		{ID: "sess-2", StartedAt: at(2), Resumed: domain.BoolFalse},
		{ID: "sess-3"},
	}
}

func richTurns(agent domain.ActorRef) []domain.Turn {
	agentNoType := domain.AgentInstance(agent.Instance)
	return []domain.Turn{
		{Actor: domain.Orchestrator(), Role: domain.TurnUser, At: at(3)},
		{Actor: agentNoType, Role: domain.TurnAssistant, At: at(4)},
		{Actor: domain.Orchestrator()}, // role and time absent
	}
}

func richNotifications(agent domain.ActorRef) []domain.Notification {
	return []domain.Notification{
		{
			Actor: domain.Orchestrator(), At: at(70), Type: "permission_prompt",
			RequiresResponse: domain.BoolTrue, Resolution: "approved", ID: "n-1",
		},
		{Actor: domain.AgentInstance(agent.Instance), At: at(71), RequiresResponse: domain.BoolFalse},
		{Actor: domain.Orchestrator()}, // everything absent
	}
}

// sparseRun is a named run where the log carried almost nothing.
func sparseRun() domain.RunAggregate {
	return domain.RunAggregate{
		Run:          domain.NamedRun(runB),
		Provisional:  true,
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator()},
	}
}

// bucketRun is the unattributable bucket.
func bucketRun() *domain.RunAggregate {
	return &domain.RunAggregate{
		Run:          domain.UnattributableRun(),
		Provisional:  true,
		Orchestrator: domain.ActorAggregate{Actor: domain.Orchestrator(), Invocations: legacyUsage()},
		Facts:        domain.RunFacts{Harnesses: []domain.Harness{domain.HarnessOpenCode}, Outcome: "failure"},
		Turns:        []domain.Turn{{Actor: domain.Orchestrator(), Role: domain.TurnAssistant, At: at(5)}},
	}
}

// fullDigest is a digest covering every carried fact, with leaky finding fields set.
func fullDigest() domain.Digest {
	agent := domain.ActorRef{Kind: domain.ActorAgentInstance, Instance: "coder#1", Type: "coder"}
	return domain.Digest{
		Header: domain.DigestHeader{ToolVersion: "1.2.3", WrittenAt: at(500)},
		Aggregate: domain.Aggregate{
			Runs:           []domain.RunAggregate{richRun(), sparseRun()},
			Unattributable: bucketRun(),
		},
		Findings: []domain.Finding{
			{Kind: domain.FindingMalformedLine, Severity: domain.SeverityWarning, Run: domain.NamedRun(runA), Actor: agent, Path: "logs/x.jsonl", Line: 7, Detail: "bad json"},
			{Kind: domain.FindingMissingModel, Severity: domain.SeverityInfo, Run: domain.UnattributableRun()},
			{Kind: domain.FindingUnrecognisedHarness, Severity: domain.SeverityInfo, Path: "logs/y.jsonl"},
		},
	}
}
