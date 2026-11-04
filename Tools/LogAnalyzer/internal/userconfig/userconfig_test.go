package userconfig_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mosaic-log-analyzer/internal/userconfig"
)

func TestDir_JoinsMosaicLogAnalyzerUnderTheBaseFolder(t *testing.T) {
	base := t.TempDir()

	got, err := userconfig.Dir(func() (string, error) { return base, nil })

	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if want := filepath.Join(base, "MOSAIC", "LogAnalyzer"); got != want {
		t.Errorf("Dir = %q, want %q", got, want)
	}
}

func TestDir_NeverCreatesTheFolder(t *testing.T) {
	base := t.TempDir()

	got, err := userconfig.Dir(func() (string, error) { return base, nil })

	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if _, statErr := os.Stat(got); !os.IsNotExist(statErr) {
		t.Errorf("Dir must not create %s; stat err = %v", got, statErr)
	}
}

func TestDir_BaseFolderUnresolvable_ReturnsErrorAndNoPath(t *testing.T) {
	boom := errors.New("no config folder")

	got, err := userconfig.Dir(func() (string, error) { return "", boom })

	if err == nil {
		t.Fatal("Dir must report that the base folder is unavailable")
	}
	if got != "" {
		t.Errorf("Dir = %q, want an empty path on error", got)
	}
}

func TestDir_EmptyBaseFolder_IsUnavailable(t *testing.T) {
	got, err := userconfig.Dir(func() (string, error) { return "", nil })

	if err == nil || got != "" {
		t.Errorf("Dir = (%q, %v), want an error and no path (a relative folder would depend on the working directory)", got, err)
	}
}
