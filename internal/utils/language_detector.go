package utils

import (
	"github.com/pemistahl/lingua-go"
	"strings"
)

var languageDetector lingua.LanguageDetector

// InitLanguageDetector initializes the language detector with common languages
func InitLanguageDetector() {
	languageDetector = lingua.NewLanguageDetectorBuilder().
		FromAllLanguages().
		Build()
}

// DetectLanguage detects the language of the given text
// Returns ISO 639-1 language code (e.g., "jp", "th", "ge", "en")
func DetectLanguage(text string) string {
	// Initialize detector if it hasn't been initialized yet
	if languageDetector == nil {
		InitLanguageDetector()
	}

	// Detect language
	language, exists := languageDetector.DetectLanguageOf(text)

	if !exists {
		// Default to English if no language is detected
		return "en"
	}

	// Return ISO 639-1 code
	return strings.ToLower(language.IsoCode639_1().String())
}

// GetDefaultTranscriptionType returns the default transcription type for the given language code
func GetDefaultTranscriptionType(languageCode string) string {
	switch languageCode {
	case "jp":
		return "furigana"
	case "zh":
		return "pinyin"
	case "th":
		return "thai_romanization"
	case "ge":
		return "mkhedruli"
	default:
		return "none"
	}
}
