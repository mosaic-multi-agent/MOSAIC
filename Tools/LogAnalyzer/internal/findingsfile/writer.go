// Package findingsfile is the I/O adapter that writes the findings export
// document of the findings overlay.
package findingsfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"mosaic-log-analyzer/internal/domain"
)

// Writer implements domain.FindingsFileWriter.
type Writer struct{}

// NewWriter returns a Writer.
func NewWriter() *Writer { return &Writer{} }

// entry is the wire shape of one finding. Unlike the report's JSON, no field is
// omitted when empty.
type entry struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	RunID    string `json:"run_id"`
	Actor    string `json:"actor"`
	Detail   string `json:"detail"`
}

// document is the wire shape of the findings file.
type document struct {
	Counts   map[string]int `json:"counts"`
	Findings []entry        `json:"findings"`
}

func encode(findings []domain.Finding) ([]byte, error) {
	doc := document{Counts: map[string]int{}, Findings: make([]entry, 0, len(findings))}
	for _, f := range findings {
		doc.Counts[string(f.Kind)]++
		doc.Findings = append(doc.Findings, entry{
			Kind:     string(f.Kind),
			Severity: f.Severity.Label(),
			Path:     f.Path,
			Line:     f.Line,
			RunID:    f.Run.Label(),
			Actor:    f.Actor.Label(),
			Detail:   f.Detail,
		})
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// WriteFindings implements domain.FindingsFileWriter.
func (w *Writer) WriteFindings(ctx context.Context, path string, findings []domain.Finding, overwrite bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !overwrite {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("%s: %w", path, domain.ErrFindingsFileExists)
		}
	}
	data, err := encode(findings)
	if err != nil {
		return fmt.Errorf("encode findings: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create findings directory: %w", err)
	}
	tmp, err := writeTemp(dir, data)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	return publish(tmp, path, overwrite)
}

// writeTemp writes data to a new temporary file in dir and returns its name.
func writeTemp(dir string, data []byte) (string, error) {
	f, err := os.CreateTemp(dir, ".findings-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary findings file: %w", err)
	}
	name := f.Name()
	_, werr := f.Write(data)
	cerr := f.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("write findings file: %w", err)
	}
	return name, nil
}

// publish moves the finished temporary file to path. Without overwrite it never
// replaces an existing file, even one that appeared since the first check.
func publish(tmp, path string, overwrite bool) error {
	if overwrite {
		if err := os.Rename(tmp, path); err != nil {
			return fmt.Errorf("replace findings file: %w", err)
		}
		return nil
	}
	if err := os.Link(tmp, path); err == nil {
		return nil
	}
	if _, statErr := os.Lstat(path); statErr == nil {
		return fmt.Errorf("%s: %w", path, domain.ErrFindingsFileExists)
	}
	// The filesystem cannot hard-link; the existence check above still holds.
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("write findings file: %w", err)
	}
	return nil
}

var _ domain.FindingsFileWriter = (*Writer)(nil)
