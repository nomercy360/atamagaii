package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
		"jp": "Japanese",
		"en": "English",
		"es": "Spanish",
		"ru": "Russian",
		"ko": "Korean",
		"fr": "French",
		"de": "German",
		"it": "Italian",
		"pt": "Portuguese",
		"ar": "Arabic",
		"tr": "Turkish",
		"th": "Thai",
		"hi": "Hindi",
		"ge": "Georgian",
		"vi": "Vietnamese",
	}

	if name, ok := languages[code]; ok {
		return name
	}
	return "Unknown"
}

// GetDefaultTranscriptionType returns the default transcription type for the given language code
func GetDefaultTranscriptionType(languageCode string) string {
	switch languageCode {
	case "jp":
		return "furigana"

	case "th":
		return "thai_romanization"
	case "ge":
		return "mkhedruli"
	default:
		return "none"
	}
}

// RemoveFurigana removes furigana notation (text inside square brackets) from a string
// For example: "今日[きょう]は良い[いい]天気[てんき]です" -> "今日は良い天気です"
// Also handles nested brackets: "複雑[ふく[ざつ]]な例" -> "複雑な例"
func RemoveFurigana(text string) string {
	// Handle nested brackets - keep replacing until no more changes
	oldText := ""
	newText := text

	for oldText != newText {
		oldText = newText
		re := regexp.MustCompile(`\[[^\[\]]*\]`)
		newText = re.ReplaceAllString(oldText, "")
	}

	return newText
}
