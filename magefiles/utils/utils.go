package utils

import (
	"os"
)

const (
	tmpPathPrefix = "bootc-images-"

	// ReadWriteExecuteUser is used for any directory or executable not normally used by the end user or containing sensitive data
	ReadWriteExecuteUser = 0700
)

// MakeTempDir creates a temp directory with the zarf- prefix.
func MakeTempDir(basePath string) (string, error) {
	if basePath != "" {
		if err := CreateDirectory(basePath, ReadWriteExecuteUser); err != nil {
			return "", err
		}
	}
	tmp, err := os.MkdirTemp(basePath, tmpPathPrefix)
	if err != nil {
		return "", err
	}
	return tmp, nil
}

// CreateDirectory creates a directory for the given path and file mode.
func CreateDirectory(path string, mode os.FileMode) error {
	if InvalidPath(path) {
		return os.MkdirAll(path, mode)
	}
	return nil
}

// InvalidPath checks if the given path is valid (if it is a permissions error it is there we just don't have access)
func InvalidPath(path string) bool {
	_, err := os.Stat(path)
	return !os.IsPermission(err) && err != nil
}
