package digest

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Version is the digest_version this package writes. MAJOR.MINOR.PATCH.
const Version = "1.0.0"

// formatName is the value of the top-level "format" key.
const formatName = "mosaic-run-digest"

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// majorOf returns the major number of a MAJOR.MINOR.PATCH version.
func majorOf(version string) (int, bool) {
	if !versionPattern.MatchString(version) {
		return 0, false
	}
	major, err := strconv.Atoi(strings.SplitN(version, ".", 2)[0])
	return major, err == nil
}

// Wire values of closed vocabularies.
const (
	stateOpen   = "open"
	stateClosed = "closed"

	pairingPaired    = "paired"
	pairingStartOnly = "start_only"
	pairingEndOnly   = "end_only"

	severityInfo    = "info"
	severityWarning = "warning"

	scopeRun            = "run"
	scopeUnattributable = "unattributable"
	scopeGeneral        = "general"

	actorOrchestrator  = "orchestrator"
	actorAgentInstance = "agent_instance"
)

// The types below are the digest file format. Only keys declared here are ever
// written; the encoder never marshals domain structs directly.

type digestDoc struct {
	Format         string       `json:"format"`
	DigestVersion  string       `json:"digest_version"`
	ToolVersion    string       `json:"tool_version,omitempty"`
	WrittenAt      string       `json:"written_at,omitempty"`
	Runs           []runDoc     `json:"runs"`
	Unattributable *runDoc      `json:"unattributable,omitempty"`
	Findings       []findingDoc `json:"findings,omitempty"`
}

type runDoc struct {
	RunID                string            `json:"run_id,omitempty"`
	Provisional          bool              `json:"provisional"`
	Harnesses            []string          `json:"harnesses,omitempty"`
	SchemaVersions       []string          `json:"schema_versions,omitempty"`
	AdapterVersion       string            `json:"adapter_version,omitempty"`
	OrchestratorModel    string            `json:"orchestrator_model,omitempty"`
	OrchestratorProvider string            `json:"orchestrator_provider,omitempty"`
	StartedAt            string            `json:"started_at,omitempty"`
	EndedAt              string            `json:"ended_at,omitempty"`
	Outcome              string            `json:"outcome,omitempty"`
	Sessions             []sessionDoc      `json:"sessions,omitempty"`
	Orchestrator         orchestratorDoc   `json:"orchestrator"`
	Agents               []agentDoc        `json:"agents,omitempty"`
	Invocations          []invocationDoc   `json:"invocations,omitempty"`
	Turns                []turnDoc         `json:"turns,omitempty"`
	Notifications        []notificationDoc `json:"notifications,omitempty"`
	Compactions          []compactionDoc   `json:"compactions,omitempty"`
}

type sessionDoc struct {
	SessionID string `json:"session_id,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	EndedAt   string `json:"ended_at,omitempty"`
	Resumed   *bool  `json:"resumed,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Model     string `json:"model,omitempty"`
	Provider  string `json:"provider,omitempty"`
}

type orchestratorDoc struct {
	Usage     []usageDoc    `json:"usage,omitempty"`
	ToolCalls []toolCallDoc `json:"tool_calls,omitempty"`
}

type agentDoc struct {
	InstanceID string     `json:"instance_id,omitempty"`
	AgentType  string     `json:"agent_type,omitempty"`
	Usage      []usageDoc `json:"usage,omitempty"`
}

type invocationDoc struct {
	InstanceID string        `json:"instance_id,omitempty"`
	AgentType  string        `json:"agent_type,omitempty"`
	State      string        `json:"state"`
	StatusCode string        `json:"status_code,omitempty"`
	Model      string        `json:"model,omitempty"`
	Provider   string        `json:"provider,omitempty"`
	StartedAt  string        `json:"started_at,omitempty"`
	EndedAt    string        `json:"ended_at,omitempty"`
	EndEvents  int           `json:"end_events"`
	ToolCalls  []toolCallDoc `json:"tool_calls,omitempty"`
}

type usageDoc struct {
	RecordID    string     `json:"record_id,omitempty"`
	At          string     `json:"at,omitempty"`
	Model       string     `json:"model,omitempty"`
	Provider    string     `json:"provider,omitempty"`
	Effort      string     `json:"effort,omitempty"`
	ServiceTier string     `json:"service_tier,omitempty"`
	Harness     string     `json:"harness,omitempty"`
	Tokens      *tokensDoc `json:"tokens,omitempty"`
}

type tokensDoc struct {
	Input           *int64 `json:"input_tokens,omitempty"`
	CacheRead       *int64 `json:"cache_read_tokens,omitempty"`
	CacheCreation   *int64 `json:"cache_creation_tokens,omitempty"`
	CacheCreation5m *int64 `json:"cache_creation_5m_tokens,omitempty"`
	CacheCreation1h *int64 `json:"cache_creation_1h_tokens,omitempty"`
	Output          *int64 `json:"output_tokens,omitempty"`
	Reasoning       *int64 `json:"reasoning_tokens,omitempty"`
}

type toolCallDoc struct {
	CallID    string `json:"call_id,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
	Status    string `json:"status,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	EndedAt   string `json:"ended_at,omitempty"`
	Pairing   string `json:"pairing"`
}

type turnDoc struct {
	Actor *actorDoc `json:"actor"`
	Role  string    `json:"role,omitempty"`
	At    string    `json:"at,omitempty"`
}

type notificationDoc struct {
	Actor            *actorDoc `json:"actor"`
	At               string    `json:"at,omitempty"`
	NotificationType string    `json:"notification_type,omitempty"`
	RequiresResponse *bool     `json:"requires_response,omitempty"`
	Resolution       string    `json:"resolution,omitempty"`
	NotificationID   string    `json:"notification_id,omitempty"`
}

type compactionDoc struct {
	Actor        *actorDoc `json:"actor"`
	At           string    `json:"at,omitempty"`
	Trigger      string    `json:"trigger,omitempty"`
	TokensBefore *int64    `json:"tokens_before,omitempty"`
	TokensAfter  *int64    `json:"tokens_after,omitempty"`
}

type actorDoc struct {
	Kind       string `json:"kind"`
	InstanceID string `json:"instance_id,omitempty"`
	AgentType  string `json:"agent_type,omitempty"`
}

type findingDoc struct {
	Kind     string    `json:"kind"`
	Severity string    `json:"severity"`
	Scope    string    `json:"scope"`
	RunID    string    `json:"run_id,omitempty"`
	Actor    *actorDoc `json:"actor"`
}

// formatTime renders a time as RFC 3339 with nanoseconds, offset preserved;
// the zero time is absent ("").
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}

// parseTime reads a formatted time; absent or unparseable text is the zero time.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
