// Package userconfig resolves the LogAnalyzer global configuration folder.
package userconfig

import (
	"errors"
	"path/filepath"
)

// Dir returns the LogAnalyzer global config folder under the folder base returns
// (production: os.UserConfigDir). It never creates the folder. An unresolvable
// or empty base folder is an error and yields no path, because a relative folder
// would depend on the working directory.
func Dir(base func() (string, error)) (string, error) {
	root, err := base()
	if err != nil {
		return "", err
	}
	if root == "" {
		return "", errors.New("user config folder is empty")
	}
	return filepath.Join(root, "MOSAIC", "LogAnalyzer"), nil
}
