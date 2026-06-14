package copier

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrCloneUnsupported = errors.New("clone unsupported")

func CopyFile(src string, dst string, mode fs.FileMode) error {
	return copyFileWithClone(src, dst, mode, cloneFile)
}

func copyFileWithClone(src string, dst string, mode fs.FileMode, clone func(string, string) error) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := clone(src, dst); err != nil {
		if !errors.Is(err, ErrCloneUnsupported) {
			return err
		}
		if err := normalCopy(src, dst, mode); err != nil {
			return err
		}
		return os.Chmod(dst, mode)
	}
	return os.Chmod(dst, mode)
}

func normalCopy(src string, dst string, mode fs.FileMode) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
