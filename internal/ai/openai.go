package ai

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"atamagaii/internal/utils"
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

func (c *OpenAIClient) GenerateCardContent(ctx context.Context, term string, language string) (*contract.CardFields, error) {
	openAIDir, err := utils.FindDirUp("data", 3)
	if err != nil {
		return nil, fmt.Errorf("failed to find materials directory: %w", err)
	}

	filePath := filepath.Join(openAIDir, "templates", language, "generate_card.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file %s does not exist (looked in %s)", filePath, openAIDir)
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	text := string(fileData)
	text = strings.ReplaceAll(text, "{{term}}", term)

	payload := strings.NewReader(text)

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/responses", payload)

	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	type Response struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
		}
	}

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error: received status code %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	response := &Response{}
	if err := json.Unmarshal(responseBody, response); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	if len(response.Output) == 0 || len(response.Output[0].Content) == 0 {
		return nil, fmt.Errorf("no content found in response")
	}

	var vocabularyCard contract.CardFields
	if err := json.Unmarshal([]byte(response.Output[0].Content[0].Text), &vocabularyCard); err != nil {
		return nil, fmt.Errorf("error parsing card fields: %w", err)
	}

	return &vocabularyCard, nil
}

// GenerateTask creates a task for the specified card
func (c *OpenAIClient) GenerateTask(ctx context.Context, card *db.Card, templateName string) (*db.TaskContent, error) {
	// Extract vocabulary item from card fields
	var vocabItem db.VocabularyItem
	if err := json.Unmarshal([]byte(card.Fields), &vocabItem); err != nil {
		return nil, fmt.Errorf("error unmarshaling card fields: %w", err)
	}

	// Determine language code to use for template
	languageCode := "ja" // Default to Japanese
	if vocabItem.LanguageCode != "" {
		// Map language code from the card to template directory format
		switch vocabItem.LanguageCode {
		case "jp", "ja":
			languageCode = "ja"
		case "ka", "ge":
			languageCode = "ka"
		case "th":
			languageCode = "th"
		}
	}

	// Find template directory
	openAIDir, err := utils.FindDirUp("data", 3)
	if err != nil {
		return nil, fmt.Errorf("failed to find templates directory: %w", err)
	}

	// Load template file
	filePath := filepath.Join(openAIDir, "templates", languageCode, templateName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("template file %s does not exist (looked in %s)", filePath, openAIDir)
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading template file %s: %w", filePath, err)
	}

	// Construct target word from vocabulary item
	targetWord := vocabItem.Term
	if vocabItem.MeaningEn != "" {
		targetWord = fmt.Sprintf("%s (%s) %s", vocabItem.MeaningEn, vocabItem.TermWithTranscription, vocabItem.Term)
	}

	// Replace placeholders in template
	text := string(fileData)
	text = strings.ReplaceAll(text, "{{targetWord}}", targetWord)
	text = strings.ReplaceAll(text, "{{knownWords}}", "") // Could be populated with user's known words in the future

	// Prepare and send request
	payload := strings.NewReader(text)
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/responses", payload)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	// Define response structure
	type Response struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
		}
	}

	// Execute request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error: received status code %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Parse response
	response := &Response{}
	if err := json.Unmarshal(responseBody, response); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	if len(response.Output) == 0 || len(response.Output[0].Content) == 0 {
		return nil, fmt.Errorf("no content found in response")
	}

	// Parse task content
	var taskContent db.TaskContent
	if err := json.Unmarshal([]byte(response.Output[0].Content[0].Text), &taskContent); err != nil {
		return nil, fmt.Errorf("error parsing task content: %w", err)
	}

	return &taskContent, nil
}

func (c *OpenAIClient) GenerateAudio(ctx context.Context, text string, language string) (string, error) {
	params := openai.AudioSpeechNewParams{
		Model:          openai.SpeechModelGPT4oMiniTTS,
		Input:          text,
		Voice:          openai.AudioSpeechNewParamsVoiceOnyx,
		ResponseFormat: openai.AudioSpeechNewParamsResponseFormatAAC,
		Speed:          openai.Float(1.3),
		Instructions:   openai.String(getAudioInstructions(language)),
	}

	response, err := c.client.Audio.Speech.New(ctx, params)

	if err != nil {
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
