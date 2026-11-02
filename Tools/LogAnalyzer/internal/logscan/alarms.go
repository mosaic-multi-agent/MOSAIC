package logscan

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

const (
	// diagnosticsFileName is the adapter diagnostics file in the logs root.
	diagnosticsFileName = "mosaic-logger-diagnostics.jsonl"
	// alarmKind is the diagnostics kind written when a transcript had assistant
	// turns and no usage records.
	alarmKind = "transcript_format_changed"
	// maxDiagnosticsLine bounds one diagnostics line.
	maxDiagnosticsLine = 1 << 20
)

// wireDiagnostic is the part of a diagnostics line the scanner reads.
type wireDiagnostic struct {
	Timestamp string `json:"timestamp"`
	Harness   string `json:"harness"`
	Kind      string `json:"kind"`
	RunID     string `json:"run_id"`
	SessionID string `json:"session_id"`
}

// TranscriptAlarms implements domain.AlarmSource: it reads the diagnostics file of
// the source's logs root once and returns its format-change alarms in file order.
func (s *Scanner) TranscriptAlarms(ctx context.Context, src domain.Source) []domain.TranscriptAlarm {
	var root string
	switch src.Kind {
	case domain.SourceLogsRoot:
		root = src.Path
	case domain.SourceSingleRun:
		root = filepath.Dir(src.Path)
	default:
		return nil
	}
	path := filepath.Join(root, diagnosticsFileName)
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var alarms []domain.TranscriptAlarm
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxDiagnosticsLine)
	for line := 1; scanner.Scan(); line++ {
		if ctx.Err() != nil {
			return alarms
		}
		var d wireDiagnostic
		if json.Unmarshal(scanner.Bytes(), &d) != nil || d.Kind != alarmKind || !domain.IsRunID(d.RunID) {
			continue
		}
		alarms = append(alarms, domain.TranscriptAlarm{
			Run:       domain.NamedRun(d.RunID),
			Harness:   domain.Harness(d.Harness),
			SessionID: d.SessionID,
			At:        parseAlarmTime(d.Timestamp),
			Path:      path,
			Line:      line,
		})
	}
	return alarms
}

func parseAlarmTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

var _ domain.AlarmSource = (*Scanner)(nil)
