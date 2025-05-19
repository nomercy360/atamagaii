package ai

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/utils"
	"bytes"
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
	apiKey string
	client *openai.Client
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

	return &OpenAIClient{
		apiKey: apiKey,
		client: &client,
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

	return vocabularyCard, nil
}

func (c *OpenAIClient) GenerateTask(ctx context.Context, language, templateName string, targetWord *string, knownWords []string) (interface{}, error) {
	fileData, err := loadTemplateFile(language, templateName)
	if err != nil {
		return nil, err
	}

	text := string(fileData)
	if targetWord != nil {
		text = strings.ReplaceAll(text, "{{targetWord}}", *targetWord)
	}

	text = strings.ReplaceAll(text, "{{knownWords}}", strings.Join(knownWords, ", "))

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
	params := openai.AudioSpeechNewParams{
		Model:          openai.SpeechModelGPT4oMiniTTS,
		Input:          text,
		Voice:          openai.AudioSpeechNewParamsVoiceOnyx,
		ResponseFormat: openai.AudioSpeechNewParamsResponseFormatAAC,
		Speed:          openai.Float(1.5),
		Instructions:   openai.String(getAudioInstructions(language)),
	}

	response, err := c.client.Audio.Speech.New(ctx, params)
	if err != nil {
		// Try to extract detailed error information from the SDK error
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "status code:") {
			return "", fmt.Errorf("error generating audio (API error): %s", errorMsg)
		}
		return "", fmt.Errorf("error generating audio: %w", err)
	}

	tempFile, err := os.CreateTemp("", "audio-*.mp3")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %w", err)
	}
	defer tempFile.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}
	defer response.Body.Close()

	if _, err := tempFile.Write(body); err != nil {
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

func getAudioInstructions(language string) string {
	switch language {
	case "japanese":
		return "Speak in natural Japanese with clear pronunciation."
	case "georgian":
		return "Speak in natural Georgian with clear pronunciation."
	case "thai":
		return "Speak in natural Thai with clear pronunciation."
	default:
		return "Speak with clear pronunciation."
	}
}
