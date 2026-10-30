package cli

import (
	"fmt"
	"io"

	"mosaic-log-analyzer/internal/domain"
)

// printConfigPaths prints one line per configuration level. A store that reports
// only a project file (no global information) prints just that path.
func printConfigPaths(w io.Writer, p domain.ConfigPaths) {
	if p.Global.Path == "" && p.Global.Unavailable == "" {
		fmt.Fprintln(w, p.Project.Path)
		return
	}
	if p.Global.Path == "" {
		fmt.Fprintf(w, "global:  (unavailable: %s)\n", p.Global.Unavailable)
	} else {
		fmt.Fprintf(w, "global:  %s (%s)\n", p.Global.Path, existence(p.Global))
	}
	fmt.Fprintf(w, "project: %s (%s)\n", p.Project.Path, existence(p.Project))
}

func existence(f domain.ConfigFile) string {
	if f.Exists {
		return "exists"
	}
	return "absent"
}
