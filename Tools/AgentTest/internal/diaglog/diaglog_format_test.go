package diaglog_test

// Tests for how Sink.Log renders an event: its fields, multi-line messages
// and arrival order.

import (
	"os"
	"path/filepath"
	"testing"

	commonharness "mosaic-common/harness"

	"mosaic-agent-test/internal/diaglog"
)

const formatRunID = "20260101T120000Z-0001"

// logEvents writes events through a fresh Sink and returns the file content
// that follows the attribution header line.
func logEvents(t *testing.T, events ...commonharness.Event) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "diag.log")
	sink, err := diaglog.Open(path, formatRunID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, ev := range events {
		sink.Log(ev)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	header := "# run: " + formatRunID + "\n"
	content := string(data)
	if len(content) < len(header) || content[:len(header)] != header {
		t.Fatalf("file does not start with the run header %q; got:\n%s", header, content)
	}
	return content[len(header):]
}

func fields(kv ...string) []commonharness.Field {
	var out []commonharness.Field
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, commonharness.Field{Key: kv[i], Value: kv[i+1]})
	}
	return out
}

func TestSink_Log_RendersFieldsInEventOrder(t *testing.T) {
	got := logEvents(t, commonharness.Event{
		Name:    "spawn.end",
		Message: "process ended: exit",
		Fields:  fields("reason", "exit", "stdout_bytes", "5120", "exit_code", "0"),
	})

	want := "spawn.end reason=exit stdout_bytes=5120 exit_code=0: process ended: exit\n"
	if got != want {
		t.Errorf("rendered entry = %q, want %q", got, want)
	}
}

func TestSink_Log_QuotesFieldValuesThatAreAmbiguous(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"empty", "", `k=""`},
		{"whitespace", "a b", `k="a b"`},
		{"colon", "a:b", `k="a:b"`},
		{"equals", "a=b", `k="a=b"`},
		{"quote", `a"b`, `k="a\"b"`},
		{"plain", "plain-1.5s", `k=plain-1.5s`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := logEvents(t, commonharness.Event{
				Name: "spawn.x", Message: "m", Fields: fields("k", tc.value),
			})

			want := "spawn.x " + tc.want + ": m\n"
			if got != want {
				t.Errorf("rendered entry = %q, want %q", got, want)
			}
		})
	}
}

func TestSink_Log_EmptyMessageWithFieldsEndsAfterColon(t *testing.T) {
	got := logEvents(t, commonharness.Event{
		Name: "spawn.stderr", Message: "", Fields: fields("bytes", "0"),
	})

	want := "spawn.stderr bytes=0:\n"
	if got != want {
		t.Errorf("rendered entry = %q, want %q", got, want)
	}
}

func TestSink_Log_EventWithoutFieldsKeepsPlainFormat(t *testing.T) {
	got := logEvents(t, commonharness.Event{Name: "spawn.timeout", Message: "invocation exceeded configured timeout"})

	want := "spawn.timeout: invocation exceeded configured timeout\n"
	if got != want {
		t.Errorf("rendered entry = %q, want %q", got, want)
	}
}

func TestSink_Log_MultiLineMessageIsBlockWithEndMarker(t *testing.T) {
	got := logEvents(t, commonharness.Event{
		Name:    "spawn.stdout.lines",
		Message: "first line\nsecond line",
		Fields:  fields("first", "1", "lines", "2"),
	})

	want := "spawn.stdout.lines first=1 lines=2:\nfirst line\nsecond line\n--- end spawn.stdout.lines ---\n"
	if got != want {
		t.Errorf("rendered entry = %q, want %q", got, want)
	}
}

func TestSink_Log_MultiLineMessageWithTrailingNewlineAddsNoBlankLine(t *testing.T) {
	got := logEvents(t, commonharness.Event{
		Name:    "spawn.stderr",
		Message: "err one\nerr two\n",
		Fields:  fields("bytes", "16"),
	})

	want := "spawn.stderr bytes=16:\nerr one\nerr two\n--- end spawn.stderr ---\n"
	if got != want {
		t.Errorf("rendered entry = %q, want %q", got, want)
	}
}

func TestSink_Log_KeepsArrivalOrder(t *testing.T) {
	got := logEvents(t,
		commonharness.Event{Name: "spawn.stdout.lines", Message: "a", Fields: fields("first", "1", "lines", "1")},
		commonharness.Event{Name: "spawn.stdout.lines", Message: "b", Fields: fields("first", "2", "lines", "1")},
		commonharness.Event{Name: "spawn.end", Message: "process ended: exit", Fields: fields("reason", "exit")},
	)

	want := "spawn.stdout.lines first=1 lines=1: a\n" +
		"spawn.stdout.lines first=2 lines=1: b\n" +
		"spawn.end reason=exit: process ended: exit\n"
	if got != want {
		t.Errorf("file content = %q, want %q", got, want)
	}
}
