package harness_test

// Tests for OpenCodeV2Env: the child's PWD must equal its working directory so
// a stale inherited PWD can never make V2 resolve the wrong project.

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"mosaic-common/harness"
)

// pwdEntries returns every environment entry whose key is PWD (matched the way
// the platform matches environment keys).
func pwdEntries(env []string) []string {
	var out []string
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		if key == "PWD" || (runtime.GOOS == "windows" && strings.EqualFold(key, "PWD")) {
			out = append(out, e)
		}
	}
	return out
}

func TestOpenCodeV2Env_AppendsPWDEqualToWorkingDir(t *testing.T) {
	dir := t.TempDir()

	env := harness.OpenCodeV2Env(dir, nil)

	want := []string{"PWD=" + dir}
	if !reflect.DeepEqual(env, want) {
		t.Errorf("want %v, got %v", want, env)
	}
}

func TestOpenCodeV2Env_PWDEntryIsLast(t *testing.T) {
	dir := t.TempDir()

	env := harness.OpenCodeV2Env(dir, []string{"A=1", "B=2"})

	want := []string{"A=1", "B=2", "PWD=" + dir}
	if !reflect.DeepEqual(env, want) {
		t.Errorf("want %v, got %v", want, env)
	}
}

func TestOpenCodeV2Env_RemovesStalePWDEntriesAndKeepsTheRestInOrder(t *testing.T) {
	dir := t.TempDir()

	env := harness.OpenCodeV2Env(dir, []string{"A=1", "PWD=/stale/one", "B=2", "PWD=/stale/two", "C=3"})

	want := []string{"A=1", "B=2", "C=3", "PWD=" + dir}
	if !reflect.DeepEqual(env, want) {
		t.Errorf("want %v, got %v", want, env)
	}
	if got := pwdEntries(env); len(got) != 1 {
		t.Errorf("want exactly one PWD entry, got %v", got)
	}
}

func TestOpenCodeV2Env_PWDKeyCaseFollowsPlatform(t *testing.T) {
	dir := t.TempDir()

	env := harness.OpenCodeV2Env(dir, []string{"pwd=/stale"})

	if runtime.GOOS == "windows" {
		// Windows environment keys are case-insensitive: the stale entry goes.
		if want := []string{"PWD=" + dir}; !reflect.DeepEqual(env, want) {
			t.Errorf("want %v, got %v", want, env)
		}
		return
	}
	// Elsewhere keys are case-sensitive: lower-case pwd is a different variable.
	if want := []string{"pwd=/stale", "PWD=" + dir}; !reflect.DeepEqual(env, want) {
		t.Errorf("want %v, got %v", want, env)
	}
}

func TestOpenCodeV2Env_PWDValuePrefixIsNotConfusedWithKey(t *testing.T) {
	dir := t.TempDir()

	env := harness.OpenCodeV2Env(dir, []string{"PWDX=keep", "XPWD=keep", "PWD_OLD=keep"})

	want := []string{"PWDX=keep", "XPWD=keep", "PWD_OLD=keep", "PWD=" + dir}
	if !reflect.DeepEqual(env, want) {
		t.Errorf("want %v, got %v", want, env)
	}
}

func TestOpenCodeV2Env_RelativeWorkingDirBecomesAbsolute(t *testing.T) {
	env := harness.OpenCodeV2Env("some-relative-dir", nil)

	abs, err := filepath.Abs("some-relative-dir")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	if want := []string{"PWD=" + abs}; !reflect.DeepEqual(env, want) {
		t.Errorf("want %v, got %v", want, env)
	}
}

func TestOpenCodeV2Env_EmptyWorkingDirUsesProcessDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}

	env := harness.OpenCodeV2Env("", []string{"A=1", "PWD=/stale"})

	want := []string{"A=1", "PWD=" + cwd}
	if !reflect.DeepEqual(env, want) {
		t.Errorf("want %v, got %v", want, env)
	}
}

func TestOpenCodeV2Env_DoesNotModifyInput(t *testing.T) {
	in := []string{"A=1", "PWD=/stale", "B=2"}
	snapshot := append([]string(nil), in...)

	harness.OpenCodeV2Env(t.TempDir(), in)

	if !reflect.DeepEqual(in, snapshot) {
		t.Errorf("want input untouched %v, got %v", snapshot, in)
	}
}

func TestOpenCodeV2Env_ResultIsNeverEmptyWhenDirectoryKnown(t *testing.T) {
	// Run builds an explicit environment only for a non-empty Env, so the
	// override can only take effect when the result is non-empty.
	if env := harness.OpenCodeV2Env(t.TempDir(), nil); len(env) == 0 {
		t.Error("want a non-empty result for a known working directory")
	}
	if env := harness.OpenCodeV2Env("", nil); len(env) == 0 {
		t.Error("want a non-empty result for the process directory")
	}
}
