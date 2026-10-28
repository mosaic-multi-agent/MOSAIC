package otelingest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"mosaic-common/otelingest/diag"
)

const noticesFileName = "notices.json"

// noticeRecord is .otel-ingest/notices.json: when a notice of each kind was first reported for the run.
type noticeRecord struct {
	Reported map[string]string `json:"reported"`
}

// ReportOnce records that a once-per-run notice of kind is being reported for the run and tells the caller
// whether it is the first. When the record cannot be read or written it reports true: a duplicate notice is
// preferred over a lost one.
func ReportOnce(runDir string, kind diag.Kind) bool {
	path := filepath.Join(runDir, stateDirName, noticesFileName)
	rec := noticeRecord{Reported: map[string]string{}}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if json.Unmarshal(data, &rec) != nil {
			return true
		}
		if rec.Reported == nil {
			rec.Reported = map[string]string{}
		}
	case !errors.Is(err, os.ErrNotExist):
		return true
	}
	if _, done := rec.Reported[string(kind)]; done {
		return false
	}
	rec.Reported[string(kind)] = time.Now().UTC().Format(lockTimeFormat)
	_ = writeNotices(path, rec)
	return true
}

// writeNotices saves the record atomically; the run folder itself must already exist.
func writeNotices(path string, rec noticeRecord) error {
	dir := filepath.Dir(path)
	if err := os.Mkdir(dir, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "notices-*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		return errors.Join(werr, cerr)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
