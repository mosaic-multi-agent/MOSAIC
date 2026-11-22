package codexsetup

// helpers_test.go holds the shared fixtures of the Codex setup checker tests.
//
// Home configurations live in testdata/homes/<name>.toml and project
// configurations in testdata/projects/<name>.toml. Workspace paths are not
// known up front, so fixtures carry placeholders that the helpers replace:
// {{WORKDIR}}, {{PARENT}}, {{GRANDPARENT}}, {{OTHER}}, {{SIBLING}},
// {{WORKDIR_UPPER}}, {{WORKDIR_SLASH}}, {{WORKDIR_SLASH_UPPER}} and
// {{WORKDIR_TRAILING}}.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fixtureEnv is a throwaway Codex home plus workspace.
type fixtureEnv struct {
	root      string // grandparent of the workspace
	parent    string // parent of the workspace
	workDir   string // the checked workspace
	codexHome string // value of CODEX_HOME when set
	userHome  string // value of the injected home directory
}

// newFixtureEnv creates root/parent/ws plus empty CODEX_HOME and user home
// directories. Path comparison is case-sensitive unless a test overrides it.
func newFixtureEnv(t *testing.T) *fixtureEnv {
	t.Helper()
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	workDir := filepath.Join(parent, "ws")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	e := &fixtureEnv{
		root:      root,
		parent:    parent,
		workDir:   workDir,
		codexHome: filepath.Join(root, "codex-home"),
		userHome:  filepath.Join(root, "user-home"),
	}
	for _, d := range []string{e.codexHome, e.userHome} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

// render replaces the placeholders of a fixture.
func (e *fixtureEnv) render(raw string) string {
	r := strings.NewReplacer(
		"{{WORKDIR_UPPER}}", strings.ToUpper(e.workDir),
		"{{WORKDIR_SLASH_UPPER}}", strings.ToUpper(filepath.ToSlash(e.workDir)),
		"{{WORKDIR_SLASH}}", filepath.ToSlash(e.workDir),
		"{{WORKDIR_TRAILING}}", e.workDir+string(filepath.Separator),
		"{{WORKDIR}}", e.workDir,
		"{{GRANDPARENT}}", e.root,
		"{{PARENT}}", e.parent,
		"{{SIBLING}}", e.workDir+"-other",
		"{{OTHER}}", filepath.Join(e.root, "elsewhere"),
	)
	return r.Replace(raw)
}

func readFixture(t *testing.T, kind, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", kind, name+".toml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// installHome writes the rendered home fixture as $CODEX_HOME/config.toml and
// returns its path.
func (e *fixtureEnv) installHome(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(e.codexHome, "config.toml")
	if err := os.WriteFile(path, []byte(e.render(readFixture(t, "homes", name))), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// installProject writes the project fixture as <workDir>/.codex/config.toml.
func (e *fixtureEnv) installProject(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(e.workDir, ".codex")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(e.render(readFixture(t, "projects", name))), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// checker builds a checker that sees CODEX_HOME = e.codexHome and the user
// home e.userHome. Case-sensitive unless ci is true.
func (e *fixtureEnv) checker(ci bool) *Checker {
	return NewChecker(
		WithGetenv(func(k string) string {
			if k == "CODEX_HOME" {
				return e.codexHome
			}
			return ""
		}),
		WithUserHomeDir(func() (string, error) { return e.userHome, nil }),
		WithCaseInsensitive(ci),
	)
}

// checkHome installs the named home fixture and checks the workspace.
func (e *fixtureEnv) checkHome(t *testing.T, name string, ci bool) Result {
	t.Helper()
	e.installHome(t, name)
	return e.checker(ci).Check(e.workDir)
}

// snapshotTree returns every file under dirs as "path=content", sorted, so a
// test can prove a check changed nothing.
func snapshotTree(t *testing.T, dirs ...string) []string {
	t.Helper()
	var out []string
	for _, d := range dirs {
		err := filepath.WalkDir(d, func(p string, de os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if de.IsDir() {
				out = append(out, p+"/")
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			out = append(out, p+"="+string(b))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(out)
	return out
}
