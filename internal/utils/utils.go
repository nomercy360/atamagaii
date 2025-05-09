package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

func FindDirUp(dirName string, maxDepth int) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	for i := 0; i <= maxDepth; i++ {
		checkPath := filepath.Join(wd, dirName)
		if stat, err := os.Stat(checkPath); err == nil && stat.IsDir() {
			return checkPath, nil
		}
		wd = filepath.Dir(wd)
	}

	return "", fmt.Errorf("directory %q not found within %d levels up", dirName, maxDepth)
}
