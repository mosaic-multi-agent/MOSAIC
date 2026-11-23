package fatal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestFailureFile_FailWritesAllFields(t *testing.T) {
	g := newRig(t, nil)

	guard(t, func() { g.r.Fail("error: load catalog: boom") })

	_, got := onlyFailureFile(t, g.dir)
	want := "MOSAIC failure report\n" +
		"Time: 2026-11-23T17:31:44.123Z\n" +
		"Tool: mosaic-deploy 1.14.0\n" +
		"Command line: " + testExe + " render --output json\n" +
		"Kind: failure\n" +
		"Exit code: 1\n" +
		"\n" +
		"Message:\n" +
		"error: load catalog: boom"
	if strings.TrimRight(got, "\n") != want {
		t.Fatalf("failure file content:\n%s\nwant:\n%s", got, want)
	}
}

func TestFailureFile_CommandLineQuotesAwkwardArguments(t *testing.T) {
	g := newRig(t, func(c *Config) {
		c.Args = []string{"tool", "a b", "", `say"hi"`, "plain"}
	})

	guard(t, func() { g.r.Fail("error: x") })

	_, got := onlyFailureFile(t, g.dir)
	want := `Command line: tool "a b" "" "say\"hi\"" plain`
	if !strings.Contains(got, want+"\n") {
		t.Fatalf("command line not quoted as expected, want %q in:\n%s", want, got)
	}
}

func TestFailureFile_NameFollowsPattern(t *testing.T) {
	g := newRig(t, nil)

	guard(t, func() { g.r.Fail("error: x") })

	path, _ := onlyFailureFile(t, g.dir)
	want := fmt.Sprintf("mosaic-deploy-failure-20261123T173144.123Z-%d.txt", os.Getpid())
	if filepath.Base(path) != want {
		t.Fatalf("file name = %q, want %q", filepath.Base(path), want)
	}
}

func TestFailureFile_ConcurrentFailuresInOneFolderGetSeparateFiles(t *testing.T) {
	g := newRig(t, nil)
	other := newRig(t, nil)
	other.cfg.FailureDir = func() (string, error) { return g.dir, nil }
	second := Install(other.cfg)

	guard(t, func() { g.r.Fail("error: first") })
	guard(t, func() { second.Fail("error: second") })

	files := failureFiles(t, g.dir)
	if len(files) != 2 {
		t.Fatalf("want 2 separate failure files, got %v", files)
	}
	var all strings.Builder
	for _, f := range files {
		b, _ := os.ReadFile(f)
		all.Write(b)
	}
	if !strings.Contains(all.String(), "error: first") || !strings.Contains(all.String(), "error: second") {
		t.Fatalf("both messages must survive, got:\n%s", all.String())
	}
	re := regexp.MustCompile(`-\d+-2\.txt$`)
	hits := 0
	for _, f := range files {
		if re.MatchString(f) {
			hits++
		}
	}
	if hits != 1 {
		t.Fatalf("the colliding name must get a -2 suffix before .txt, files: %v", files)
	}
}

func TestFailureFile_ExistingFileIsNeverOverwritten(t *testing.T) {
	g := newRig(t, nil)
	taken := filepath.Join(g.dir, fmt.Sprintf("mosaic-deploy-failure-20261123T173144.123Z-%d.txt", os.Getpid()))
	if err := os.WriteFile(taken, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}

	guard(t, func() { g.r.Fail("error: x") })

	b, _ := os.ReadFile(taken)
	if string(b) != "precious" {
		t.Fatalf("existing file was overwritten: %q", b)
	}
	if n := len(failureFiles(t, g.dir)); n != 2 {
		t.Fatalf("want the original plus one new file, got %d", n)
	}
}

func TestFailureFile_UnresolvableLocationShowsOnlyOriginalMessage(t *testing.T) {
	g := newRig(t, func(c *Config) {
		c.FailureDir = func() (string, error) { return "", os.ErrNotExist }
	})

	guard(t, func() { g.r.Fail("error: original") })

	if got := g.stderr.String(); got != "error: original\n" {
		t.Fatalf("stderr = %q, want only the original message", got)
	}
	if codes := g.exits.Codes(); !equalInts(codes, []int{1}) {
		t.Fatalf("exit codes = %v, want [1]", codes)
	}
}

func TestFailureFile_UnwritableLocationShowsOnlyOriginalMessage(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no", "such", "folder")
	g := newRig(t, func(c *Config) {
		c.FailureDir = func() (string, error) { return missing, nil }
	})

	guard(t, func() { g.r.Fail("error: original") })

	if got := g.stderr.String(); got != "error: original\n" {
		t.Fatalf("stderr = %q, want only the original message", got)
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatalf("the reporter must not create the failure folder")
	}
}

func TestFailureFile_NoFileOnSuccessOrDocumentedOutcome(t *testing.T) {
	for _, code := range []int{0, 1, 2, 3, 4} {
		g := newRig(t, nil)

		guard(t, func() { g.r.Exit(code) })

		if files := failureFiles(t, g.dir); len(files) != 0 {
			t.Fatalf("Exit(%d) wrote %v", code, files)
		}
		if g.stderr.String() != "" {
			t.Fatalf("Exit(%d) printed %q", code, g.stderr.String())
		}
	}
}

func TestFailureFile_TimeAndNameAreUTCWhateverTheClockZone(t *testing.T) {
	zone := time.FixedZone("plus2", 2*3600)
	g := newRig(t, func(c *Config) {
		c.Now = func() time.Time { return fixedNow.In(zone) }
	})

	guard(t, func() { g.r.Fail("error: x") })

	path, file := onlyFailureFile(t, g.dir)
	if !strings.Contains(file, "Time: 2026-11-23T17:31:44.123Z\n") {
		t.Fatalf("Time must be UTC:\n%s", file)
	}
	if !strings.Contains(path, "failure-20261123T173144.123Z-") {
		t.Fatalf("name must carry the UTC time: %q", path)
	}
}
