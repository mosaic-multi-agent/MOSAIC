package repair

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"

	"mosaic-run/internal/dispatchlog"
	"mosaic-run/internal/domain"
)

// MaxConsultations is the number of repair consultations allowed per damage
// occurrence.
const MaxConsultations = 2

// OccurrenceInput is what one damage occurrence is opened with.
type OccurrenceInput struct {
	Store               domain.ArtifactStore        // the run's store (reads clear its guard)
	ArtifactPath        string                      // absolute Orchestration.md path
	RunID               string                      // the run's identity
	Damage              *domain.ArtifactDamageError // the read that opened the occurrence; non-nil
	LastGoodCopy        string                      // "" = none
	DamagedCopy         string                      // "" when the save failed
	ReportedDispatchLog string                      // Deps.DispatchLog.Path(); may be ""
}

// Occurrence is one damage occurrence: from the read that found the damage to
// an accepted repair or the end of the run. Not safe for concurrent use.
type Occurrence struct {
	in          OccurrenceInput
	dispatchLog string                         // usable run dispatch log; "" = none
	attempts    int                            // repair consultations counted
	latest      []domain.ArtifactDamageProblem // problems of the latest known damage
}

// NewOccurrence opens an occurrence.
func NewOccurrence(in OccurrenceInput) *Occurrence {
	return &Occurrence{
		in:          in,
		dispatchLog: usableDispatchLog(in),
		latest:      slices.Clone(in.Damage.Problems),
	}
}

// usableDispatchLog returns the run's own dispatch log when it exists: the
// path the logger reports if the reader recognises it as this run's log, else
// the run's log at its derived location. "" when neither is a log with entries.
func usableDispatchLog(in OccurrenceInput) string {
	if dispatchlog.IsRunLog(in.ReportedDispatchLog, in.RunID) && isUsableLog(in.ReportedDispatchLog) {
		return in.ReportedDispatchLog
	}
	if in.ArtifactPath == "" || in.RunID == "" {
		return ""
	}
	workspace := filepath.Dir(filepath.Dir(in.ArtifactPath))
	if derived := dispatchlog.RunLogPath(workspace, in.RunID); isUsableLog(derived) {
		return derived
	}
	return ""
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// isUsableLog reports whether path is a file that can be a dispatch log: it is
// empty, or at least one of its lines is a JSON object. A file with content
// but no JSON line tells nothing about the run.
func isUsableLog(path string) bool {
	if !isFile(path) {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	r := bufio.NewReader(f)
	sawContent := false
	for {
		line, readErr := r.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			sawContent = true
			var obj map[string]json.RawMessage
			if json.Unmarshal(trimmed, &obj) == nil {
				return true
			}
		}
		if readErr != nil {
			return !sawContent && errors.Is(readErr, io.EOF)
		}
	}
}

// NextReport returns the artifact_damage content for the next repair
// consultation and true, counting it; zero and false once MaxConsultations
// consultations were counted.
func (o *Occurrence) NextReport() (domain.ArtifactDamageReport, bool) {
	if o.attempts >= MaxConsultations {
		return domain.ArtifactDamageReport{}, false
	}
	o.attempts++
	return domain.ArtifactDamageReport{
		Problems:     slices.Clone(o.latest),
		LastGoodCopy: o.in.LastGoodCopy,
		DispatchLog:  o.dispatchLog,
	}, true
}

// Attempt returns the number of repair consultations counted so far.
func (o *Occurrence) Attempt() int { return o.attempts }

// Judge verifies the artifact after a repair consultation that returned a reply.
func (o *Occurrence) Judge() Verdict {
	verdict := Verify(VerifyInput{
		Store:        o.in.Store,
		LastGoodCopy: o.in.LastGoodCopy,
		DispatchLog:  o.dispatchLog,
	})
	if !verdict.Accepted {
		o.latest = slices.Clone(verdict.Problems)
	}
	return verdict
}

// Unrepaired builds the error that ends the run without an accepted repair.
func (o *Occurrence) Unrepaired(reason domain.UnrepairedReason, cause error) *domain.UnrepairedDamageError {
	return &domain.UnrepairedDamageError{
		Reason:        reason,
		Artifact:      o.in.ArtifactPath,
		Problems:      slices.Clone(o.latest),
		DamagedCopy:   o.in.DamagedCopy,
		LastGoodCopy:  o.in.LastGoodCopy,
		Consultations: o.attempts,
		Cause:         cause,
	}
}
