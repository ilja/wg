//go:build !darwin && !linux

package copier

func cloneFile(src string, dst string) error {
	return ErrCloneUnsupported
}
