package utils

import (
	"os"
	"strings"
)

// GetAbsHomePath replaces ~ with the absolute path to a user's home dir
func GetAbsHomePath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		homePath, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return strings.Replace(path, "~", homePath, 1), nil
	}
	return path, nil
}
