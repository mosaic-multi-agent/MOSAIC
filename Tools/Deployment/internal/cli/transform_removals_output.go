package cli

// transform_removals_output.go lists the files a pair transform removes or replaces in the
// human transform output.

import (
	"fmt"
	"io"
	"strings"

	"mosaic-deploy/internal/app"
)

// writeTransformChanges lists the removals and then the replacements of a run. Nothing is
// written for an empty list.
func writeTransformChanges(out io.Writer, result app.TransformHarnessResult) {
	writeTransformChangeSection(out, "removals", "remove", result.Removals)
	writeTransformChangeSection(out, "replacements", "replace", result.Replacements)
}

func writeTransformChangeSection(out io.Writer, header, verb string, changes []app.TransformFileChange) {
	if len(changes) == 0 {
		return
	}
	fmt.Fprintf(out, "%s:\n", header)
	for _, c := range changes {
		fmt.Fprintln(out, transformChangeLine(verb, c))
		if c.BackupPath != "" {
			fmt.Fprintf(out, "    backup: %s\n", c.BackupPath)
		}
	}
}

// transformChangeLine is the one-line entry of a removed or replaced file.
func transformChangeLine(verb string, c app.TransformFileChange) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  %s %s", verb, c.Path)
	if c.Key != "" {
		b.Reset()
		fmt.Fprintf(&b, "  %s %s:%s %s", verb, c.Kind, c.Key, c.Path)
	}
	if c.Reason != "" {
		fmt.Fprintf(&b, " - %s", c.Reason)
	}
	return b.String()
}

// transformChangeCounts is the removed/replaced suffix of the counts line; empty when the run
// has neither.
func transformChangeCounts(result app.TransformHarnessResult) string {
	var b strings.Builder
	if n := len(result.Removals); n > 0 {
		fmt.Fprintf(&b, "  removed: %d", n)
	}
	if n := len(result.Replacements); n > 0 {
		fmt.Fprintf(&b, "  replaced: %d", n)
	}
	return b.String()
}
