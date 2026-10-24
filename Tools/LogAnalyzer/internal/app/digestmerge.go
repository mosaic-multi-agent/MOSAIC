package app

import (
	"sort"
	"time"

	"mosaic-log-analyzer/internal/domain"
)

// runCopy is one digest's copy of a run (or of the unattributable bucket).
type runCopy struct {
	path      string
	writtenAt time.Time
	run       domain.RunAggregate
}

// copyKey identifies a digest file's copy of a run, for dropping the findings
// that belong to a copy which lost.
type copyKey struct {
	path string
	run  domain.RunRef
}

// mergeResult is the merged aggregate plus what the merge decided.
type mergeResult struct {
	aggregate domain.Aggregate
	// duplicates holds one finding per discarded copy.
	duplicates []domain.Finding
	// carried holds the digests' carried findings, minus those scoped to a
	// discarded copy.
	carried []domain.Finding
}

// mergeDigests merges the digest runs into the log aggregate. For a run id (or
// the bucket) present more than once, log data wins; among digests the one
// with the latest written-at wins, a missing written-at ranks oldest and
// equal values go to the digest that was discovered first. Every discarded
// copy yields one duplicate_run finding. Runs stay sorted by id and the bucket
// stays separate.
func mergeDigests(logs domain.Aggregate, digests []loadedDigest) mergeResult {
	named := make(map[string][]runCopy)
	var bucket []runCopy
	for _, ld := range digests {
		at := ld.digest.Header.WrittenAt
		for _, r := range ld.digest.Aggregate.Runs {
			named[r.Run.ID] = append(named[r.Run.ID], runCopy{path: ld.path, writtenAt: at, run: r})
		}
		if b := ld.digest.Aggregate.Unattributable; b != nil {
			bucket = append(bucket, runCopy{path: ld.path, writtenAt: at, run: *b})
		}
	}

	var res mergeResult
	lost := make(map[copyKey]bool)

	logRuns := make(map[string]domain.RunAggregate, len(logs.Runs))
	for _, r := range logs.Runs {
		logRuns[r.Run.ID] = r
	}
	ids := make([]string, 0, len(logRuns)+len(named))
	for id := range logRuns {
		ids = append(ids, id)
	}
	for id := range named {
		if _, ok := logRuns[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	runs := make([]domain.RunAggregate, 0, len(ids))
	for _, id := range ids {
		logRun, inLogs := logRuns[id]
		winner, losers := pickWinner(inLogs, named[id])
		if inLogs {
			runs = append(runs, logRun)
		} else {
			runs = append(runs, winner.run)
		}
		res.recordLosers(domain.NamedRun(id), losers, lost)
	}
	res.aggregate.Runs = runs

	res.aggregate.Unattributable = logs.Unattributable
	winner, losers := pickWinner(logs.Unattributable != nil, bucket)
	if logs.Unattributable == nil && len(bucket) > 0 {
		b := winner.run
		res.aggregate.Unattributable = &b
	}
	res.recordLosers(domain.UnattributableRun(), losers, lost)

	for _, ld := range digests {
		for _, f := range ld.carried {
			if f.Run != (domain.RunRef{}) && lost[copyKey{path: ld.path, run: f.Run}] {
				continue
			}
			res.carried = append(res.carried, f)
		}
	}
	return res
}

// pickWinner chooses the digest copy to use. When logs already hold the run,
// every digest copy loses and the returned winner is unused.
func pickWinner(inLogs bool, copies []runCopy) (runCopy, []runCopy) {
	if len(copies) == 0 {
		return runCopy{}, nil
	}
	if inLogs {
		return runCopy{}, copies
	}
	best := 0
	for i := 1; i < len(copies); i++ {
		if copies[i].writtenAt.After(copies[best].writtenAt) {
			best = i
		}
	}
	losers := make([]runCopy, 0, len(copies)-1)
	for i, c := range copies {
		if i != best {
			losers = append(losers, c)
		}
	}
	return copies[best], losers
}

// recordLosers raises one duplicate_run finding per discarded copy and marks
// the copy as lost.
func (m *mergeResult) recordLosers(run domain.RunRef, losers []runCopy, lost map[copyKey]bool) {
	for _, c := range losers {
		lost[copyKey{path: c.path, run: run}] = true
		m.duplicates = append(m.duplicates, domain.Finding{
			Kind:     domain.FindingDuplicateRun,
			Severity: domain.SeverityInfo,
			Run:      run,
			Path:     c.path,
		})
	}
}
