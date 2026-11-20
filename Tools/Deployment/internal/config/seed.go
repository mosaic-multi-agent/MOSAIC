package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// seededConfigNames are the config files SeedConfigFiles creates from templates.
var seededConfigNames = []string{"user-config.yaml", "tool-config.yaml"}

// SeedConfigFiles creates each of <mosaicRoot>/MosaicDeploy/config/user-config.yaml and
// <mosaicRoot>/MosaicDeploy/config/tool-config.yaml that does not exist, copying the
// byte-identical content of the file with the same name at the root of templates.
//
// Anything that exists at a destination path is left untouched. A template that is absent
// from templates (or a nil templates) creates nothing and raises no error. Both names are
// always attempted; the returned error joins every I/O failure.
func SeedConfigFiles(mosaicRoot string, templates fs.FS) error {
	if templates == nil {
		return nil
	}
	var errs []error
	for _, name := range seededConfigNames {
		if err := seedOne(mosaicRoot, templates, name); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// seedOne seeds a single config file from its template, if the destination is absent.
func seedOne(mosaicRoot string, templates fs.FS, name string) error {
	dest := filepath.Join(mosaicRoot, "MosaicDeploy", "config", name)
	if _, err := os.Lstat(dest); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("check %s: %w", dest, err)
	}

	info, err := fs.Stat(templates, name)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	data, err := fs.ReadFile(templates, name)
	if err != nil {
		return fmt.Errorf("read template %s: %w", name, err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create config directory for %s: %w", dest, err)
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil
		}
		return fmt.Errorf("create %s: %w", dest, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", dest, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	return nil
}
