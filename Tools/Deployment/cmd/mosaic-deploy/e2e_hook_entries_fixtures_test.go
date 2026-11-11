package main

// e2e_hook_entries_fixtures_test.go holds the fixtures of the per-file hook entry tests: a
// tool-local hook bundle with a Claude Code variant (a flat file and a nested file, plus one
// registration step) and an OpenCode variant (one plugin file), a selections file that deploys
// it with a workflow, and readers for the manifest the binary writes. Nothing in this file is a
// test itself.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
)

const (
	hkKey        = "pf-logger"
	hkFlatRel    = ".claude/hooks/pf_flat.py"
	hkNestedRel  = ".claude/hooks/lib/pf_nested.py"
	hkRegRel     = ".claude/settings.json"
	hkPluginRel  = ".opencode/plugins/pf_plugin.js"
	hkFlatSrc    = "print('flat')\n"
	hkNestedSrc  = "print('nested')\n"
	hkPluginSrc  = "export default {}\n"
	hkEditedText = "print('edited by hand')\n"
	hkEmptyHash  = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// hkBundleYAML is the bundle descriptor at the given version.
func hkBundleYAML(version string) string {
	return "schema_version: \"1\"\nid: " + hkKey + "\nversion: " + version + "\ndescription: Logger.\nplaceholder: false\n" +
		"variants:\n" +
		"  claude-code:\n    supported: true\n    files:\n" +
		"      - source: pf_flat.py\n        target: pf_flat.py\n" +
		"      - source: lib/pf_nested.py\n        target: lib/pf_nested.py\n" +
		"    registration:\n" +
		"      - id: register-pf-logger\n        target_path: .claude/settings.json\n        performable: true\n" +
		"        instruction: Register the logger.\n        fragment: '{\"hooks\": {}}'\n" +
		"  opencode:\n    supported: true\n    files:\n" +
		"      - source: pf_plugin.js\n        target: pf_plugin.js\n    registration: []\n"
}

// hkWriteBundle writes the bundle into the default catalog of the mosaic root at the version.
func hkWriteBundle(t *testing.T, mosaicRoot, version string) {
	t.Helper()
	def := filepath.Join(mosaicRoot, "Catalog")
	writeStackFile(t, def, filepath.Join("Hooks", hkKey, "hook.yaml"), []byte(hkBundleYAML(version)))
	writeStackFile(t, def, filepath.Join("Hooks", hkKey, "claude-code", "pf_flat.py"), []byte(hkFlatSrc))
	writeStackFile(t, def, filepath.Join("Hooks", hkKey, "claude-code", "lib", "pf_nested.py"), []byte(hkNestedSrc))
	writeStackFile(t, def, filepath.Join("Hooks", hkKey, "opencode", "pf_plugin.js"), []byte(hkPluginSrc))
}

// hkWorkspace is a mosaic root carrying the bundle and an empty workspace.
type hkWorkspace struct {
	mosaicRoot string
	ws         string
}

func newHkWorkspace(t *testing.T) hkWorkspace {
	t.Helper()
	root := newStackMosaicRoot(t)
	hkWriteBundle(t, root, "1.0.0")
	return hkWorkspace{mosaicRoot: root, ws: t.TempDir()}
}

// deployWithHook runs a deploy of the default workflow and the hook bundle and returns the run
// summary.
func (h hkWorkspace) deployWithHook(t *testing.T) hkSummary {
	t.Helper()
	sel := filepath.Join(t.TempDir(), "selections.yaml")
	if err := os.WriteFile(sel, []byte("workflows:\n  - default-wf\nhooks:\n  - "+hkKey+"\n"), 0o644); err != nil {
		t.Fatalf("write selections: %v", err)
	}
	return h.run(t, "deploy", "--selections", sel, "--auto-confirm", "--output", "json")
}

// run executes the binary for the workspace and decodes the JSON run summary.
func (h hkWorkspace) run(t *testing.T, args ...string) hkSummary {
	t.Helper()
	run := runInWorkspace(t, h.mosaicRoot, h.ws, args...)
	requireRanOrSkipped(t, run)
	var s hkSummary
	if err := json.Unmarshal([]byte(run.Stdout), &s); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\nstdout: %s", err, run.Stdout)
	}
	return s
}

// requireRanOrSkipped fails unless the run completed, with or without skipped files (exit code 0
// or 2); the tests assert on the workspace, so a skipped file shows up there.
func requireRanOrSkipped(t *testing.T, run binaryRun) {
	t.Helper()
	if run.ExitCode != 0 && run.ExitCode != 2 {
		t.Fatalf("exit code %d, want 0 or 2\nstdout: %s\nstderr: %s", run.ExitCode, run.Stdout, run.Stderr)
	}
}

// hkSummary is the part of the run summary the hook entry tests read.
type hkSummary struct {
	Actions []struct {
		Ref        struct{ Kind, Key string }
		TargetPath string
		Taken      string
	}
}

// takenOf returns the Taken values of the actions of the kind.
func (s hkSummary) takenOf(kind string) []string {
	var out []string
	for _, a := range s.Actions {
		if a.Ref.Kind == kind {
			out = append(out, a.Taken)
		}
	}
	return out
}

// hkEntry is one manifest entry as the tests read it.
type hkEntry struct {
	Harness string `yaml:"harness"`
	Ref     struct {
		Kind string `yaml:"kind"`
		Key  string `yaml:"key"`
	} `yaml:"ref"`
	TargetPath  string `yaml:"target_path"`
	ContentHash string `yaml:"content_hash"`
}

// hookEntries returns the hook entries of the workspace manifest by harness and target path.
func (h hkWorkspace) hookEntries(t *testing.T) map[string]hkEntry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.ws, ".mosaic", "manifest.yaml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var doc struct {
		Entries []hkEntry `yaml:"entries"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	out := map[string]hkEntry{}
	for _, e := range doc.Entries {
		if e.Ref.Kind == "hook" {
			out[e.Harness+"|"+e.TargetPath] = e
		}
	}
	return out
}

func (h hkWorkspace) read(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.ws, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func (h hkWorkspace) write(t *testing.T, rel, text string) {
	t.Helper()
	writeStackFile(t, h.ws, filepath.FromSlash(rel), []byte(text))
}

func (h hkWorkspace) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(h.ws, filepath.FromSlash(rel)))
	return err == nil
}

// backups returns the files under .mosaic/backups.
func (h hkWorkspace) backups() []string {
	var out []string
	_ = filepath.Walk(filepath.Join(h.ws, ".mosaic", "backups"), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func (h hkWorkspace) readAbs(t *testing.T, abs string) string {
	t.Helper()
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", abs, err)
	}
	return string(data)
}

func removeWorkspaceFile(ws, rel string) error {
	return os.Remove(filepath.Join(ws, filepath.FromSlash(rel)))
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// useBundleLevelRecord rewrites the manifest as a deployment made before per-file hook entries
// looks: the per-file hook entries of the bundle are replaced by one bundle-level entry at a
// path that does not exist, carrying the empty-content hash and no content record.
func (h hkWorkspace) useBundleLevelRecord(t *testing.T) {
	t.Helper()
	var doc yaml.MapSlice
	if err := yaml.UnmarshalWithOptions(readManifestBytes(t, h.ws), &doc, yaml.UseOrderedMap()); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	replaced := false
	for i, top := range doc {
		if top.Key != "entries" {
			continue
		}
		entries, _ := top.Value.([]interface{})
		var kept []interface{}
		for _, e := range entries {
			fields, ok := e.(yaml.MapSlice)
			if !ok || !hkIsHookEntry(fields) {
				kept = append(kept, e)
				continue
			}
			if replaced {
				continue
			}
			replaced = true
			bundle := yaml.MapSlice{}
			for _, f := range fields {
				switch f.Key {
				case "owned_content":
					continue
				case "target_path":
					f.Value = ".claude/hooks/" + hkKey
				case "content_hash":
					f.Value = hkEmptyHash
				}
				bundle = append(bundle, f)
			}
			kept = append(kept, bundle)
		}
		doc[i].Value = kept
	}
	if !replaced {
		t.Fatal("manifest holds no hook entry to turn into a bundle-level record")
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath(h.ws), out, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// hkIsHookEntry reports whether a manifest entry is a hook entry.
func hkIsHookEntry(fields yaml.MapSlice) bool {
	for _, f := range fields {
		if f.Key != "ref" {
			continue
		}
		ref, _ := f.Value.(yaml.MapSlice)
		for _, r := range ref {
			if r.Key == "kind" && r.Value == "hook" {
				return true
			}
		}
	}
	return false
}

func hkHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "sha256:" + hex.EncodeToString(sum[:])
}
