package activity_test

// Behaviour every decoder shares: tolerance of unknown, partial or malformed
// lines, and the short form of tool input (single line, bounded, truncation
// marked by "...").

import (
	"strings"
	"testing"
	"unicode/utf8"

	"mosaic-run/internal/domain"
	"mosaic-run/internal/harness/activity"
)

type decoderCase struct {
	name string
	new  func() activity.Decoder
	// bashLine renders a tool call of a shell tool whose command is command
	// (already JSON-escaped, without quotes).
	bashLine func(command string) string
}

var decoderCases = []decoderCase{
	{
		name: "claude-code",
		new:  activity.NewClaudeCodeDecoder,
		bashLine: func(command string) string {
			return `{"type":"assistant","message":{"id":"m","role":"assistant","content":[{"type":"tool_use","id":"tu","name":"Bash","input":{"command":"` + command + `"}}]}}`
		},
	},
	{
		name: "opencode",
		new:  activity.NewOpenCodeDecoder,
		bashLine: func(command string) string {
			return `{"type":"tool_use","sessionID":"s","part":{"type":"tool","callID":"c","tool":"bash","state":{"status":"completed","input":{"command":"` + command + `"},"output":"o"}}}`
		},
	},
	{
		name: "ghcp-cli",
		new:  activity.NewGHCPCLIDecoder,
		bashLine: func(command string) string {
			return `{"type":"tool.execution_start","data":{"toolCallId":"t","toolName":"bash","arguments":{"command":"` + command + `"}}}`
		},
	},
}

func TestDecoders_UnknownPartialOrMalformedLines_AreSkippedWithoutPanic(t *testing.T) {
	for _, tc := range decoderCases {
		t.Run(tc.name, func(t *testing.T) {
			valid := tc.bashLine("ls")
			lines := []string{
				"",
				"   ",
				"not json at all",
				`{"type":`,
				`[]`,
				`null`,
				`{}`,
				`{"type":"something.unknown","data":{}}`,
				`{"type":"assistant"}`,
				`{"type":"tool_use","part":5}`,
				`{"type":"tool.execution_start","data":"oops"}`,
				valid[:len(valid)/2], // a line cut in half
			}
			dec := tc.new()
			for _, l := range lines {
				if items := dec.Decode([]byte(l)); len(items) != 0 {
					t.Errorf("line %q: want no items, got %+v", l, items)
				}
			}
			// A decoder keeps working after bad input.
			if items := dec.Decode([]byte(valid)); len(items) != 1 || items[0].Kind != domain.ActivityToolCall {
				t.Errorf("want one tool call after malformed lines, got %+v", items)
			}
		})
	}
}

func TestDecoders_ToolInput_ShortFormIsSingleLine(t *testing.T) {
	for _, tc := range decoderCases {
		t.Run(tc.name, func(t *testing.T) {
			items := tc.new().Decode([]byte(tc.bashLine(`echo one\necho two\r\necho three`)))

			if len(items) != 1 {
				t.Fatalf("want one item, got %+v", items)
			}
			assertSingleLine(t, items[0].Summary)
			for _, part := range []string{"echo one", "echo two", "echo three"} {
				if !strings.Contains(items[0].Summary, part) {
					t.Errorf("summary %q lost %q", items[0].Summary, part)
				}
			}
		})
	}
}

func TestDecoders_ToolInput_LongInputIsTruncatedWithEllipsis(t *testing.T) {
	for _, tc := range decoderCases {
		t.Run(tc.name, func(t *testing.T) {
			items := tc.new().Decode([]byte(tc.bashLine(strings.Repeat("x", 1000))))

			if len(items) != 1 {
				t.Fatalf("want one item, got %+v", items)
			}
			assertSingleLine(t, items[0].Summary)
			if !strings.HasSuffix(items[0].Summary, "...") {
				t.Errorf("truncated summary must end with ..., got %q", items[0].Summary)
			}
			if n := len([]rune(items[0].Summary)); n < domain.ActivitySummaryMaxRunes-10 {
				t.Errorf("summary cut far below the limit: %d runes", n)
			}
		})
	}
}

func TestDecoders_ToolInput_TruncationCountsRunesNotBytes(t *testing.T) {
	for _, tc := range decoderCases {
		t.Run(tc.name, func(t *testing.T) {
			items := tc.new().Decode([]byte(tc.bashLine(strings.Repeat("é", 500))))

			if len(items) != 1 {
				t.Fatalf("want one item, got %+v", items)
			}
			assertSingleLine(t, items[0].Summary)
			if !utf8.ValidString(items[0].Summary) {
				t.Errorf("summary is not valid UTF-8: %q", items[0].Summary)
			}
			if n := len([]rune(items[0].Summary)); n < domain.ActivitySummaryMaxRunes-10 {
				t.Errorf("summary cut by bytes instead of runes: %d runes", n)
			}
		})
	}
}

func TestDecoders_ShortToolInput_IsNotTruncated(t *testing.T) {
	for _, tc := range decoderCases {
		t.Run(tc.name, func(t *testing.T) {
			items := tc.new().Decode([]byte(tc.bashLine("go vet ./cmd")))

			if len(items) != 1 {
				t.Fatalf("want one item, got %+v", items)
			}
			if strings.HasSuffix(items[0].Summary, "...") {
				t.Errorf("short input must not be marked truncated: %q", items[0].Summary)
			}
		})
	}
}
