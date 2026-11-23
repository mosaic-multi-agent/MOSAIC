package fatal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxNameAttempts = 1000

// writeFailureFile writes the failure file and returns its path. Any problem
// (unresolvable or unwritable folder) skips the file without an error.
func (r *Reporter) writeFailureFile(kind, message, stack string) (string, bool) {
	dir, err := r.cfg.FailureDir()
	if err != nil || dir == "" {
		return "", false
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	now := r.cfg.Now().UTC()
	base := fmt.Sprintf("%s-failure-%s-%d", r.cfg.Tool, now.Format("20060102T150405.000Z"), os.Getpid())
	content := r.failureFileContent(now.Format("2006-01-02T15:04:05.000Z"), kind, message, stack)

	for attempt := 1; attempt <= maxNameAttempts; attempt++ {
		name := base + ".txt"
		if attempt > 1 {
			name = fmt.Sprintf("%s-%d.txt", base, attempt)
		}
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", false
		}
		_, werr := f.WriteString(content)
		cerr := f.Close()
		if werr != nil || cerr != nil {
			return "", false
		}
		return path, true
	}
	return "", false
}

func (r *Reporter) failureFileContent(timestamp, kind, message, stack string) string {
	var b strings.Builder
	b.WriteString("MOSAIC failure report\n")
	b.WriteString("Time: " + timestamp + "\n")
	b.WriteString("Tool: " + r.cfg.Tool + " " + r.cfg.Version + "\n")
	b.WriteString("Command line: " + joinCommandLine(r.cfg.Args) + "\n")
	b.WriteString("Kind: " + kind + "\n")
	b.WriteString("Exit code: 1\n")
	b.WriteString("\nMessage:\n")
	b.WriteString(message)
	if !strings.HasSuffix(message, "\n") {
		b.WriteString("\n")
	}
	if kind == "panic" {
		b.WriteString("\nStack:\n")
		b.WriteString(stack)
		if !strings.HasSuffix(stack, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// joinCommandLine joins args with single spaces; an argument that is empty or has
// whitespace or a double quote is written in Go %q form.
func joinCommandLine(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		if a == "" || strings.ContainsAny(a, " \t\r\n\"") {
			parts[i] = fmt.Sprintf("%q", a)
		} else {
			parts[i] = a
		}
	}
	return strings.Join(parts, " ")
}
