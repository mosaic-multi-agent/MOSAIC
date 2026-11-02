package otelingest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const (
	stateDirName  = ".otel-ingest"
	stateFileName = "state.json"
	headBytes     = 4096
)

// cursor is the read position in the OTel file. head_sha256 covers the first head_len bytes of the file and
// identifies it, so a different file at the same path is noticed.
type cursor struct {
	Offset     int64  `json:"offset"`
	HeadLen    int64  `json:"head_len"`
	HeadSHA256 string `json:"head_sha256"`
}

// ingestState is the content of .otel-ingest/state.json, the contract between ingest implementations.
type ingestState struct {
	StateVersion int               `json:"state_version"`
	Cursor       *cursor           `json:"cursor,omitempty"`
	Placements   map[string]string `json:"placements"`
	Pending      []*call           `json:"pending"`
	Links        map[string]*link  `json:"links"`
}

func newIngestState() *ingestState {
	return &ingestState{
		StateVersion: StateVersion,
		Placements:   map[string]string{},
		Pending:      []*call{},
		Links:        map[string]*link{},
	}
}

func statePath(runDir string) string {
	return filepath.Join(runDir, stateDirName, stateFileName)
}

// loadState reads the run's state. present is false when there is none; unknown carries the raw version text of a
// state this package does not understand.
func loadState(runDir string) (st *ingestState, present bool, unknown *string, err error) {
	data, err := os.ReadFile(statePath(runDir))
	if errors.Is(err, os.ErrNotExist) {
		return newIngestState(), false, nil, nil
	}
	if err != nil {
		return nil, true, nil, err
	}
	var head struct {
		StateVersion json.RawMessage `json:"state_version"`
	}
	if json.Unmarshal(data, &head) != nil || string(head.StateVersion) != "1" {
		raw := string(head.StateVersion)
		return nil, true, &raw, nil
	}
	st = newIngestState()
	if err := json.Unmarshal(data, st); err != nil {
		return nil, true, nil, err
	}
	if st.Placements == nil {
		st.Placements = map[string]string{}
	}
	if st.Links == nil {
		st.Links = map[string]*link{}
	}
	if st.Pending == nil {
		st.Pending = []*call{}
	}
	return st, true, nil, nil
}

// saveState writes the state atomically (temporary file, then rename).
func saveState(runDir string, st *ingestState) error {
	dir := filepath.Join(runDir, stateDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "state-*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		return errors.Join(werr, cerr)
	}
	if err := os.Rename(tmp.Name(), statePath(runDir)); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// headDigest returns the length and SHA-256 of the first min(headBytes, size) bytes of the file.
func headDigest(path string, size int64) (int64, string, error) {
	n := min(int64(headBytes), size)
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.CopyN(h, f, n); err != nil && !errors.Is(err, io.EOF) {
		return 0, "", err
	}
	return n, hex.EncodeToString(h.Sum(nil)), nil
}

// cursorValid reports whether the stored cursor still describes the file: it is not shorter than the cursor and
// its head is unchanged.
func cursorValid(c *cursor, path string, size int64) bool {
	if c == nil || size < c.Offset || size < c.HeadLen {
		return false
	}
	n, sum, err := headDigest(path, c.HeadLen)
	return err == nil && n == c.HeadLen && sum == c.HeadSHA256
}
