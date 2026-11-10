package app

import (
	"strings"

	"mosaic-log-analyzer/internal/domain"
)

// runFolder is one run's folder in the enumerated source.
type runFolder struct {
	dir string
	run domain.RunRef
}

// exportableFindings returns the findings an export carries: the findings from
// reading the logs, with those that name no run attributed to the run folder
// they came from, followed by the findings carried inside digests. Findings
// from no run's folder stay run-less (general findings). The log report is not
// affected: it keeps the findings as raised.
func exportableFindings(inv domain.Inventory, logFindings, digestCarried []domain.Finding) []domain.Finding {
	var folders []runFolder
	for _, r := range inv.Runs {
		folders = append(folders, runFolder{dir: r.Dir, run: r.Run})
	}
	if inv.Unattributable != nil {
		folders = append(folders, runFolder{dir: inv.Unattributable.Dir, run: domain.UnattributableRun()})
	}

	out := make([]domain.Finding, 0, len(logFindings)+len(digestCarried))
	for _, f := range logFindings {
		if f.Run == (domain.RunRef{}) {
			f.Run = originRun(folders, f.Path)
		}
		out = append(out, f)
	}
	return append(out, digestCarried...)
}

// originRun names the run whose folder contains path, or the zero RunRef when
// path lies in no run's folder.
func originRun(folders []runFolder, path string) domain.RunRef {
	for _, f := range folders {
		if isWithin(f.dir, path) {
			return f.run
		}
	}
	return domain.RunRef{}
}

// isWithin reports whether path is dir itself or lies beneath it. A sibling
// that merely shares dir as a name prefix is not within it.
func isWithin(dir, path string) bool {
	if dir == "" || path == "" {
		return false
	}
	if path == dir {
		return true
	}
	rest, ok := strings.CutPrefix(path, dir)
	return ok && (rest[0] == '/' || rest[0] == 0x5c)
}
