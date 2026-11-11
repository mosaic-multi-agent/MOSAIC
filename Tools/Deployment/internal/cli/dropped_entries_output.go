package cli

// dropped_entries_output.go lists the manifest entries dropped before planning, each with its
// path and reason, in the dry-run report and the human transform output.

import (
	"fmt"
	"io"

	"mosaic-deploy/internal/app"
	"mosaic-deploy/internal/domain"
)

const droppedEntriesHeader = "dropped manifest entries:"

// writeDroppedEntries lists the dropped entries of a plan. Nothing is written for an empty list.
func writeDroppedEntries(out io.Writer, dropped []domain.DroppedEntry) {
	if len(dropped) == 0 {
		return
	}
	fmt.Fprintln(out, droppedEntriesHeader)
	for _, d := range dropped {
		fmt.Fprintf(out, "  drop %s%s\n", d.Entry.TargetPath, dryRunReason(d.Reason))
	}
}

// writeTransformDroppedEntries lists the dropped entries of a transform run. Nothing is written
// for an empty list.
func writeTransformDroppedEntries(out io.Writer, dropped []app.TransformDroppedEntry) {
	if len(dropped) == 0 {
		return
	}
	fmt.Fprintln(out, droppedEntriesHeader)
	for _, d := range dropped {
		fmt.Fprintf(out, "  drop %s%s\n", d.Path, dryRunReason(d.Reason))
	}
}
