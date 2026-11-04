package app

import (
	"fmt"
	"os"
)

// requireExistingWorkspace refuses a workspace path that does not exist or is not a
// directory. The update flows document the path as the existing workspace, so they must
// fail before any write instead of creating the folder.
func requireExistingWorkspace(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrWorkspaceNotFound, path)
		}
		return fmt.Errorf("%w: %s: %v", ErrWorkspaceNotFound, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrWorkspaceNotFound, path)
	}
	return nil
}
