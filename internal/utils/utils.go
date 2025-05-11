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

func GetLanguageNameFromCode(code string) string {
	languages := map[string]string{
		"ja": "Japanese",
		"en": "English",
		"es": "Spanish",
		"ru": "Russian",
		"zh": "Chinese",
		"ko": "Korean",
		"fr": "French",
		"de": "German",
		"it": "Italian",
		"pt": "Portuguese",
		"ar": "Arabic",
		"tr": "Turkish",
		"th": "Thai",
		"hi": "Hindi",
		"ka": "Georgian",
		"vi": "Vietnamese",
	}

	if name, ok := languages[code]; ok {
		return name
	}
	return "Unknown"
}
