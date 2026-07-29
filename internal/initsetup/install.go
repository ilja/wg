package initsetup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Install(sourcePath, destinationPath string, force bool) (returnErr error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("setup template %q does not exist; create it before running wg init: %w", sourcePath, err)
		}
		return fmt.Errorf("open setup template %q: %w", sourcePath, err)
	}
	defer func() {
		if err := source.Close(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("close setup template %q: %w", sourcePath, err)
		}
	}()

	sourceInfo, err := source.Stat()
	if err != nil {
		return fmt.Errorf("inspect setup template %q: %w", sourcePath, err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("setup template %q is not a regular file", sourcePath)
	}

	parent := filepath.Dir(destinationPath)
	if err := prepareDestinationParent(parent); err != nil {
		return err
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
	defer func() {
		if temporaryPath == "" {
			return
		}
		if err := os.Remove(temporaryPath); err != nil && !os.IsNotExist(err) && returnErr == nil {
			returnErr = fmt.Errorf("remove temporary setup hook %q: %w", temporaryPath, err)
		}
	}()

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
		temporaryPath = ""
		return nil
	}
	if err := os.Link(temporaryPath, destinationPath); err != nil {
		return fmt.Errorf("install setup hook %q: %w", destinationPath, err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return fmt.Errorf("remove temporary setup hook %q: %w", temporaryPath, err)
	}
	temporaryPath = ""
	return nil
}

func prepareDestinationParent(parent string) error {
	info, err := os.Lstat(parent)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("setup directory %q is not a directory", parent)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect setup directory %q: %w", parent, err)
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create setup directory %q: %w", parent, err)
	}
	info, err = os.Lstat(parent)
	if err != nil {
		return fmt.Errorf("inspect created setup directory %q: %w", parent, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("setup directory %q is not a directory", parent)
	}
	return nil
}
