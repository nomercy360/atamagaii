package ai

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/utils"
	"bytes"
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"context"
	"encoding/json"
	"fmt"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type OpenAIClient struct {
	apiKey    string
	client    *openai.Client
	ttsClient *texttospeech.Client
}

type OpenAIResponse struct {
	Output []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
	}
}

// OpenAIErrorResponse represents the error structure returned by OpenAI API
type OpenAIErrorResponse struct {
	Error struct {
		Message string      `json:"message"`
		Type    string      `json:"type"`
		Param   interface{} `json:"param"`
		Code    string      `json:"code"`
	} `json:"error"`
}

func NewOpenAIClient(apiKey string) (*OpenAIClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	ttsClient, err := texttospeech.NewClient(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create Google TTS client: %w", err)
	}

	return &OpenAIClient{
		apiKey:    apiKey,
		client:    &client,
		ttsClient: ttsClient,
	}, nil
}

func loadTemplateFile(languageCode, templateName string) ([]byte, error) {
	openAIDir, err := utils.FindDirUp("data", 3)
	if err != nil {
		return nil, fmt.Errorf("failed to find templates directory: %w", err)
	}

	filePath := filepath.Join(openAIDir, "templates", languageCode, templateName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("template file %s does not exist (looked in %s)", filePath, openAIDir)
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading template file %s: %w", filePath, err)
	}

	return fileData, nil
}

func (c *OpenAIClient) sendOpenAIRequest(payload []byte) (*OpenAIResponse, error) {
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse the error response
		var errorResponse OpenAIErrorResponse
		if err := json.Unmarshal(responseBody, &errorResponse); err == nil && errorResponse.Error.Message != "" {
			return nil, fmt.Errorf("OpenAI API error [%s]: %s (code: %s)", errorResponse.Error.Type, errorResponse.Error.Message, errorResponse.Error.Code)
		}

		// If we can't parse the error response, return the status code and raw body
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(responseBody))
	}

	response := &OpenAIResponse{}
	if err := json.Unmarshal(responseBody, response); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	if len(response.Output) == 0 || len(response.Output[0].Content) == 0 {
		return nil, fmt.Errorf("no content found in response")
	}

	return response, nil
}

func parseResponseContent[T any](response *OpenAIResponse) (*T, error) {
	var result T
	if err := json.Unmarshal([]byte(response.Output[0].Content[0].Text), &result); err != nil {
		return nil, fmt.Errorf("error parsing response content: %w", err)
	}

	return &result, nil
}

func (c *OpenAIClient) GenerateCardContent(ctx context.Context, term string, language string) (*contract.CardFields, error) {
	fileData, err := loadTemplateFile(language, "generate_card.json")
	if err != nil {
		return nil, err
	}

	text := string(fileData)
	text = strings.ReplaceAll(text, "{{term}}", term)

	response, err := c.sendOpenAIRequest([]byte(text))
	if err != nil {
		return nil, err
	}

	vocabularyCard, err := parseResponseContent[contract.CardFields](response)
	if err != nil {
		return nil, fmt.Errorf("error parsing card fields: %w", err)
	}

	vocabularyCard.LanguageCode = language

	return vocabularyCard, nil
}

func (c *OpenAIClient) GenerateTask(ctx context.Context, language, templateName string, targetWord string) (interface{}, error) {
	fileData, err := loadTemplateFile(language, templateName)
	if err != nil {
		return nil, err
	}

	text := string(fileData)
	fmt.Printf("Target word: %s\n", targetWord)
	text = strings.ReplaceAll(text, "{{targetWord}}", targetWord)

	response, err := c.sendOpenAIRequest([]byte(text))
	if err != nil {
		return nil, err
	}

	var rawContent map[string]interface{}
	if err := json.Unmarshal([]byte(response.Output[0].Content[0].Text), &rawContent); err != nil {
		return nil, fmt.Errorf("error parsing task content: %w", err)
	}

	return rawContent, nil
}

func (c *OpenAIClient) GenerateAudio(ctx context.Context, text string, language string) (string, error) {
	voice := getGoogleTTSVoice(language)

	req := &texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: voice.LanguageCode,
			//Name:         voice.Name,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			EffectsProfileId: []string{"handset-class-device"},
			AudioEncoding:    texttospeechpb.AudioEncoding_LINEAR16,
			SpeakingRate:     1.0,
			Pitch:            0.0,
		},
	}

	response, err := c.ttsClient.SynthesizeSpeech(ctx, req)
	if err != nil {
		return "", fmt.Errorf("error generating audio with Google TTS: %w", err)
	}

	tempFile, err := os.CreateTemp("", "audio-*.wav")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %w", err)
	}
	defer tempFile.Close()

	if _, err := tempFile.Write(response.AudioContent); err != nil {
		return "", fmt.Errorf("error writing to temp file: %w", err)
	}

	return tempFile.Name(), nil
}

type TranslationCheckResult struct {
	Score    int     `json:"score"`
	Feedback *string `json:"feedback"`
}

func (c *OpenAIClient) CheckSentenceTranslation(ctx context.Context, sentenceRu, correctAnswer, userAnswer string, languageCode string) (*TranslationCheckResult, error) {
	fileData, err := loadTemplateFile(languageCode, "check_sentence_translation.json")
	if err != nil {
		return nil, err
	}

	sentencePrompt := fmt.Sprintf("Sentence RU: %s\\nCorrect Answer: %s\\nMy Answer: %s",
		sentenceRu, correctAnswer, userAnswer)

	text := string(fileData)
	text = strings.ReplaceAll(text, "{{sentence}}", sentencePrompt)

	response, err := c.sendOpenAIRequest([]byte(text))
	if err != nil {
		return nil, err
	}

	result, err := parseResponseContent[TranslationCheckResult](response)
	if err != nil {
		return nil, fmt.Errorf("error parsing check result: %w", err)
	}

	return result, nil
}

type GoogleTTSVoice struct {
	LanguageCode string
	Name         string
}

func getGoogleTTSVoice(language string) GoogleTTSVoice {
	switch language {
	case "jp":
		return GoogleTTSVoice{
			LanguageCode: "ja-JP",
			Name:         "ja-JP-Chirp3-HD-Puck",
		}
	case "ge":
		return GoogleTTSVoice{
			LanguageCode: "ka-GE",
			Name:         "ka-GE-Standard-A",
		}
	case "th":
		return GoogleTTSVoice{
			LanguageCode: "th-TH",
			Name:         "th-TH-Standard-A",
		}
	default:
		return GoogleTTSVoice{
			LanguageCode: "en-US",
			Name:         "en-US-Standard-A",
		}
	}
}
