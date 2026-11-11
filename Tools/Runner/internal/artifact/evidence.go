package artifact

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mosaic-run/internal/domain"
)

const lastGoodName = "Orchestration.last-good.md"

// lastGoodPath returns the last-good copy path, or "" without an evidence
// location.
func (f *fileStore) lastGoodPath() string {
	dir, ok := EvidenceDir(f.path)
	if !ok {
		return ""
	}
	return filepath.Join(dir, lastGoodName)
}

// refreshLastGood keeps a copy of the bytes just written to the artifact. A
// failure is deliberately ignored: the artifact write already succeeded and the
// previous copy stays intact (the copy is written atomically).
func (f *fileStore) refreshLastGood(data []byte) {
	path := f.lastGoodPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = atomicWrite(path, data)
}

// LastGoodCopy implements domain.ArtifactEvidence.
func (f *fileStore) LastGoodCopy() (string, bool) {
	path := f.lastGoodPath()
	if path == "" {
		return "", false
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return "", false
	}
	return path, true
}

// SaveDamagedCopy implements domain.ArtifactEvidence.
func (f *fileStore) SaveDamagedCopy(now time.Time) (string, error) {
	dir, ok := EvidenceDir(f.path)
	if !ok {
		return "", domain.ErrNoEvidenceLocation
	}
	data, err := os.ReadFile(f.path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stamp := now.UTC().Format("20060102T150405Z")
	for n := 0; ; n++ {
		name := "Orchestration.damaged-" + stamp + ".md"
		if n > 0 {
			name = fmt.Sprintf("Orchestration.damaged-%s-%d.md", stamp, n)
		}
		path := filepath.Join(dir, name)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, werr := file.Write(data)
		cerr := file.Close()
		if werr != nil || cerr != nil {
			os.Remove(path)
			return "", errors.Join(werr, cerr)
		}
		return path, nil
	}
}

// ArtifactPath returns the path of the artifact file the store reads and writes.
func (f *fileStore) ArtifactPath() string {
	return f.path
}
