package opencodev2

// The set of files destined for the sandbox plugins directory, the logger
// bundle's contribution, and the tracked, sandbox-confined file writes.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mosaic-common/hookbundle"

	"mosaic-agent-test/internal/domain"
)

// PluginFile is one file destined for the sandbox's plugins directory.
type PluginFile struct {
	// Target is relative to the plugins directory; never absolute, never escaping it.
	Target  string
	Content []byte
}

// PluginContribution is one source's contribution to the plugins directory.
type PluginContribution struct {
	// Source names the contributor in a refusal message.
	Source string

	// RewritesInput declares that this contribution registers something able
	// to rewrite the intercepted call's input. At most one may.
	RewritesInput bool

	Files []PluginFile
}

// ComposePlugins collects every contribution's files into the set that will be
// written, refusing two contributions that both rewrite the call's input and
// two contributions targeting the same file.
func ComposePlugins(contribs ...PluginContribution) ([]PluginFile, error) {
	var rewriters []string
	seen := make(map[string]string)
	var files []PluginFile

	for _, c := range contribs {
		if c.RewritesInput {
			rewriters = append(rewriters, c.Source)
		}
		for _, f := range c.Files {
			if existing, ok := seen[f.Target]; ok {
				return nil, fmt.Errorf("opencode-v2: composing plugins: both %q and %q declare a file at %q", existing, c.Source, f.Target)
			}
			seen[f.Target] = c.Source
			files = append(files, f)
		}
	}
	if len(rewriters) > 1 {
		return nil, fmt.Errorf("%w: %s", ErrMultipleRewriters, strings.Join(rewriters, ", "))
	}
	return files, nil
}

// loadLoggerBundleContribution resolves the logger bundle's V2 variant through
// the shared manifest package and reads its files as data, so adding a file to
// the bundle changes what is deployed with no edit here. The registration list
// must be empty: V2 plugins are loaded by presence in the folder, so a
// non-empty list is a surprise that is refused, not ignored.
func loadLoggerBundleContribution(bundleDir string) (PluginContribution, error) {
	manifest, err := hookbundle.Load(bundleDir)
	if err != nil {
		return PluginContribution{}, fmt.Errorf("opencode-v2: loading logger bundle manifest: %w", err)
	}
	variant, err := manifest.ResolveVariant(HarnessID, "")
	if errors.Is(err, hookbundle.ErrVariantMissing) {
		// A bundle without a V2 variant offers nothing for this harness, the
		// same outcome as a variant it marks unsupported.
		return PluginContribution{}, fmt.Errorf("opencode-v2: logger bundle has no %q variant: %w", HarnessID, hookbundle.ErrVariantUnsupported)
	}
	if err != nil {
		return PluginContribution{}, fmt.Errorf("opencode-v2: resolving logger bundle variant for %q: %w", HarnessID, err)
	}
	if len(variant.Registration) != 0 {
		return PluginContribution{}, fmt.Errorf(
			"opencode-v2: logger bundle opencode-v2 variant declares a non-empty registration list (%d steps); plugins are loaded by directory presence and must never be registered",
			len(variant.Registration),
		)
	}

	variantDir := filepath.Join(bundleDir, variant.Key)
	var files []PluginFile
	for _, f := range variant.Files {
		srcPath := filepath.Join(variantDir, filepath.FromSlash(f.Source))
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return PluginContribution{}, fmt.Errorf("opencode-v2: reading logger bundle file %q: %w", srcPath, err)
		}
		files = append(files, PluginFile{Target: f.Target, Content: data})
	}
	return PluginContribution{Source: manifest.ID, Files: files}, nil
}

// safeJoin joins baseDir with a declared relative path, refusing any path that
// would resolve outside baseDir.
func safeJoin(baseDir, rel string) (string, error) {
	cleanRel := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(cleanRel) {
		return "", fmt.Errorf("target %q must be relative", rel)
	}
	full := filepath.Join(baseDir, cleanRel)

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolving %q: %w", baseDir, err)
	}
	fullAbs, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("resolving %q: %w", full, err)
	}
	relToBase, err := filepath.Rel(baseAbs, fullAbs)
	if err != nil || relToBase == ".." || strings.HasPrefix(relToBase, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("target %q escapes %q", rel, baseDir)
	}
	return full, nil
}

// writeTracked creates missing parent directories (recording exactly those),
// writes data to path and records the file, so the ledger describes precisely
// what Deprovision must remove.
func writeTracked(prov *domain.Provisioning, path string, data []byte) error {
	if err := ensureDirsFor(prov, filepath.Dir(path)); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("opencode-v2: writing %q: %w", path, err)
	}
	prov.Files = append(prov.Files, path)
	return nil
}

// ensureDirsFor creates dir and any missing ancestors, recording only the ones
// that did not exist, outermost first.
func ensureDirsFor(prov *domain.Provisioning, dir string) error {
	var missing []string
	for d := dir; ; {
		info, err := os.Stat(d)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("opencode-v2: %q exists and is not a directory", d)
			}
			break
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("opencode-v2: stat %q: %w", d, err)
		}
		missing = append(missing, d)
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0o755); err != nil && !os.IsExist(err) {
			return fmt.Errorf("opencode-v2: creating directory %q: %w", missing[i], err)
		}
		prov.Dirs = append(prov.Dirs, missing[i])
	}
	return nil
}
