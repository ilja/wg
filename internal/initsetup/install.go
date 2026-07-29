package initsetup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Install(sourcePath, destinationPath string, force bool) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open setup template %q: %w", sourcePath, err)
	}
	defer source.Close()

	sourceInfo, err := source.Stat()
	if err != nil {
		return fmt.Errorf("inspect setup template %q: %w", sourcePath, err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("setup template %q is not a regular file", sourcePath)
	}

	parent := filepath.Dir(destinationPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create setup directory %q: %w", parent, err)
	}
	destinationInfo, err := os.Lstat(destinationPath)
	if err == nil {
		if !destinationInfo.Mode().IsRegular() {
			return fmt.Errorf("setup hook %q is not a regular file", destinationPath)
		}
		if !force {
			return fmt.Errorf("setup hook %q already exists; rerun with --force to replace it", destinationPath)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect setup hook %q: %w", destinationPath, err)
	}

	temporary, err := os.CreateTemp(parent, ".setup.sh-")
	if err != nil {
		return fmt.Errorf("create temporary setup hook in %q: %w", parent, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if _, err := io.Copy(temporary, source); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("copy setup template %q: %w", sourcePath, err)
	}
	if err := temporary.Chmod(sourceInfo.Mode().Perm() | 0o100); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("make temporary setup hook executable: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary setup hook: %w", err)
	}
	if force {
		if err := os.Rename(temporaryPath, destinationPath); err != nil {
			return fmt.Errorf("replace setup hook %q: %w", destinationPath, err)
		}
		return nil
	}
	if err := os.Link(temporaryPath, destinationPath); err != nil {
		return fmt.Errorf("install setup hook %q: %w", destinationPath, err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return fmt.Errorf("remove temporary setup hook %q: %w", temporaryPath, err)
	}
	return nil
}
