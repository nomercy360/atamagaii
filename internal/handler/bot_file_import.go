package handler

import (
	"atamagaii/internal/ai"
	"atamagaii/internal/contract"
	"atamagaii/internal/utils"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// VocabImportItem represents the structure for imported vocabulary
type VocabImportItem struct {
	Term                     string `json:"term"`
	Transcription            string `json:"transcription"`
	TermWithTranscription    string `json:"term_with_transcription"`
	MeaningEn                string `json:"meaning_en"`
	MeaningRu                string `json:"meaning_ru"`
	ExampleNative            string `json:"example_native"`
	ExampleEn                string `json:"example_en"`
	ExampleRu                string `json:"example_ru"`
	ExampleWithTranscription string `json:"example_with_transcription"`
	Frequency                int    `json:"frequency"`
	LanguageCode             string `json:"language_code"`
}

// processFileImport handles the file import process
func (h *Handler) processFileImport(userID string, telegramChatID int64, document *tgbotapi.Document, messageID int) {
	ctx := context.Background()

	// Download the file
	fileContent, err := h.downloadTelegramFile(document.FileID)
	if err != nil {
		log.Printf("Failed to download file: %v", err)
		h.sendFileImportError(telegramChatID, "Не удалось загрузить файл\\. Попробуй еще раз\\.", messageID)
		return
	}

	// Parse the file based on extension
	allowedSuffixes := []string{".csv", ".txt"}
	inAllowedSuffix := false
	for _, suffix := range allowedSuffixes {
		if strings.HasSuffix(document.FileName, suffix) {
			inAllowedSuffix = true
			break
		}
	}

	if !inAllowedSuffix {
		h.sendFileImportError(telegramChatID, "Неподдерживаемый формат файла\\. Поддерживаются только CSV и TXT файлы\\.", messageID)
		return
	}

	var items []VocabImportItem
	items, err = h.parseCSVFile(ctx, fileContent)

	if err != nil {
		log.Printf("Failed to parse file: %v", err)
		h.sendFileImportError(telegramChatID, "Не удалось обработать файл\\. Проверь формат данных\\.", messageID)
		return
	}

	if len(items) == 0 {
		h.sendFileImportError(telegramChatID, "Файл не содержит данных для импорта\\.", messageID)
		return
	}

	// Update status
	h.updateImportStatus(telegramChatID, messageID, fmt.Sprintf("📝 Обработано %d записей. Создаю колоду\\.\\.\\.", len(items)))

	// Group items by language
	itemsByLang := make(map[string][]VocabImportItem)
	for _, item := range items {
		lang := item.LanguageCode
		if lang == "" {
			// Try to detect language from the term
			lang = DetectLanguageFromString(item.Term)
		}
		itemsByLang[lang] = append(itemsByLang[lang], item)
	}

	// Create decks and import cards for each language
	totalImported := 0
	var importedDecks []string

	for lang, langItems := range itemsByLang {
		transcriptionType := utils.GetDefaultTranscriptionType(lang)

		// Get or create the "Generated" deck for this language
		deck, err := h.db.GetOrCreateGeneratedDeck(userID, lang, transcriptionType)
		if err != nil {
			log.Printf("Failed to get/create deck for language %s: %v", lang, err)
			continue
		}

		// Convert items to field strings
		fieldStrings := make([]string, len(langItems))
		for i, item := range langItems {
			fields := contract.CardFields{
				Term:                     item.Term,
				Transcription:            item.Transcription,
				TermWithTranscription:    item.TermWithTranscription,
				MeaningEn:                item.MeaningEn,
				MeaningRu:                item.MeaningRu,
				ExampleNative:            item.ExampleNative,
				ExampleEn:                item.ExampleEn,
				ExampleRu:                item.ExampleRu,
				ExampleWithTranscription: item.ExampleWithTranscription,
				LanguageCode:             lang,
			}

			fieldsJSON, _ := json.Marshal(fields)
			fieldStrings[i] = string(fieldsJSON)
		}

		// Batch insert cards
		err = h.db.AddCardsInBatch(userID, deck.ID, fieldStrings)
		if err != nil {
			log.Printf("Failed to import cards for language %s: %v", lang, err)
			continue
		}

		totalImported += len(langItems)
		importedDecks = append(importedDecks, deck.ID)

		// Update status
		h.updateImportStatus(telegramChatID, messageID, fmt.Sprintf("📝 Импортировано %d карточек\\.\\.\\.", totalImported))
	}

	// Send final notification
	if totalImported > 0 {
		h.sendFileImportSuccess(telegramChatID, messageID, totalImported, importedDecks[0])
	} else {
		h.sendFileImportError(telegramChatID, "Не удалось импортировать карточки\\. Проверь формат файла\\.", messageID)
	}
}

// ColumnMapping represents the mapping of CSV columns to our fields
type ColumnMapping struct {
	TermIndex                     int
	TranscriptionIndex            int
	TermWithTranscriptionIndex    int
	MeaningEnIndex                int
	MeaningRuIndex                int
	ExampleNativeIndex            int
	ExampleEnIndex                int
	ExampleRuIndex                int
	ExampleWithTranscriptionIndex int
	FrequencyIndex                int
}

func getRandomMiddleItem[T any](items []T) T {
	if len(items) == 0 {
		var zero T
		return zero // Return zero value for type T if no items
	}

	rand.Seed(time.Now().UnixNano())

	// Define "middle" as 25% to 75%
	start := len(items) / 4
	end := 3 * len(items) / 4

	if start >= end {
		start = 0
		end = len(items)
	}

	randomIndex := rand.Intn(end-start) + start
	return items[randomIndex]
}

// parseCSVFile parses CSV file using column mapping
func (h *Handler) parseCSVFile(ctx context.Context, content []byte) ([]VocabImportItem, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	reader.Comma = '\t' // Tab-separated as in the example
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // Variable number of fields

	// Read all records first
	allRecords, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(allRecords) == 0 {
		return nil, fmt.Errorf("empty CSV file")
	}

	// Skip metadata lines at the beginning
	dataStartIndex := 0
	for i, record := range allRecords {
		if len(record) > 0 && !strings.HasPrefix(record[0], "#") || strings.HasPrefix(record[0], "Welcome") {
			dataStartIndex = i
			break
		}
	}

	if dataStartIndex >= len(allRecords) {
		return nil, fmt.Errorf("no data found in CSV file")
	}

	// get middle sample row for analysis, use getRandomMiddleItem to avoid bias
	sampleRow := getRandomMiddleItem(allRecords[dataStartIndex:])
	// If the sample row is empty, fallback to the first data row
	if len(sampleRow) == 0 {
		sampleRow = allRecords[dataStartIndex]
	}

	var line string
	for i, field := range sampleRow {
		if i > 0 {
			line += "\n"
		}
		field = strings.TrimSpace(field)
		if field == "" {
			continue // Skip empty fields
		}
		line += fmt.Sprintf("%s: %d", field, i)
	}

	analysisResult, err := h.aiClient.ParseCSVFields(ctx, line)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze columns: %w", err)
	}

	if analysisResult.LanguageCode == "" {
		return nil, fmt.Errorf("language code not detected in CSV file")
	}

	// Detect column mapping using AI
	mapping, err := h.detectColumnMapping(analysisResult)
	if err != nil {
		return nil, fmt.Errorf("failed to detect column mapping: %w", err)
	}

	// Parse all data rows using the mapping
	var items []VocabImportItem
	for i := dataStartIndex; i < len(allRecords); i++ {
		record := allRecords[i]

		// Skip empty rows
		if len(record) == 0 || (len(record) == 1 && record[0] == "") {
			continue
		}

		item := h.extractItemFromRecord(record, mapping)
		if item.Term != "" {
			items = append(items, item)
		}
	}

	return items, nil
}

// parseTxtFile parses plain text file (one term per line)
func (h *Handler) parseTxtFile(_ context.Context, content []byte) ([]VocabImportItem, error) {
	lines := strings.Split(string(content), "\n")
	var items []VocabImportItem

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Detect language
		lang := DetectLanguageFromString(line)

		items = append(items, VocabImportItem{
			Term:         line,
			LanguageCode: lang,
		})
	}

	return items, nil
}

func (h *Handler) detectColumnMapping(analysisResult ai.CSVToJSONFields) (*ColumnMapping, error) {
	// Create mapping based on field types
	mapping := &ColumnMapping{
		TermIndex:                     -1,
		TranscriptionIndex:            -1,
		TermWithTranscriptionIndex:    -1,
		MeaningEnIndex:                -1,
		MeaningRuIndex:                -1,
		ExampleNativeIndex:            -1,
		ExampleEnIndex:                -1,
		ExampleRuIndex:                -1,
		ExampleWithTranscriptionIndex: -1,
		FrequencyIndex:                -1,
	}

	if analysisResult.Term.Value != "" {
		mapping.TermIndex = analysisResult.Term.ColumnIndex
	}

	if analysisResult.Transcription.Value != "" {
		mapping.TranscriptionIndex = analysisResult.Transcription.ColumnIndex
	}

	if analysisResult.TermWithTranscription.Value != "" {
		mapping.TermWithTranscriptionIndex = analysisResult.TermWithTranscription.ColumnIndex
	}

	if analysisResult.MeaningEn.Value != "" {
		mapping.MeaningEnIndex = analysisResult.MeaningEn.ColumnIndex
	}

	if analysisResult.MeaningRu.Value != "" {
		mapping.MeaningRuIndex = analysisResult.MeaningRu.ColumnIndex
	}

	if analysisResult.ExampleNative.Value != "" {
		mapping.ExampleNativeIndex = analysisResult.ExampleNative.ColumnIndex
	}

	if analysisResult.ExampleEn.Value != "" {
		mapping.ExampleEnIndex = analysisResult.ExampleEn.ColumnIndex
	}

	if analysisResult.ExampleRu.Value != "" {
		mapping.ExampleRuIndex = analysisResult.ExampleRu.ColumnIndex
	}

	if analysisResult.ExampleWithTranscription.Value != "" {
		mapping.ExampleWithTranscriptionIndex = analysisResult.ExampleWithTranscription.ColumnIndex
	}

	if analysisResult.Frequency.Value > 0 {
		mapping.FrequencyIndex = analysisResult.Frequency.ColumnIndex
	}

	log.Printf("Column mapping detected: %+v", mapping)
	return mapping, nil
}

// extractItemFromRecord extracts a VocabImportItem from a CSV record using the column mapping
func (h *Handler) extractItemFromRecord(record []string, mapping *ColumnMapping) VocabImportItem {
	item := VocabImportItem{}

	if mapping.TermIndex >= 0 && mapping.TermIndex < len(record) {
		item.Term = strings.TrimSpace(record[mapping.TermIndex])
	}

	if mapping.TranscriptionIndex >= 0 && mapping.TranscriptionIndex < len(record) {
		item.Transcription = strings.TrimSpace(record[mapping.TranscriptionIndex])
	}

	if mapping.TermWithTranscriptionIndex >= 0 && mapping.TermWithTranscriptionIndex < len(record) {
		item.TermWithTranscription = strings.TrimSpace(record[mapping.TermWithTranscriptionIndex])
	}

	if mapping.MeaningEnIndex >= 0 && mapping.MeaningEnIndex < len(record) {
		item.MeaningEn = strings.TrimSpace(record[mapping.MeaningEnIndex])
	}

	if mapping.MeaningRuIndex >= 0 && mapping.MeaningRuIndex < len(record) {
		item.MeaningRu = strings.TrimSpace(record[mapping.MeaningRuIndex])
	}

	if mapping.ExampleNativeIndex >= 0 && mapping.ExampleNativeIndex < len(record) {
		item.ExampleNative = strings.TrimSpace(record[mapping.ExampleNativeIndex])
	}

	if mapping.ExampleEnIndex >= 0 && mapping.ExampleEnIndex < len(record) {
		item.ExampleEn = strings.TrimSpace(record[mapping.ExampleEnIndex])
	}

	if mapping.ExampleRuIndex >= 0 && mapping.ExampleRuIndex < len(record) {
		item.ExampleRu = strings.TrimSpace(record[mapping.ExampleRuIndex])
	}

	if mapping.ExampleWithTranscriptionIndex >= 0 && mapping.ExampleWithTranscriptionIndex < len(record) {
		item.ExampleWithTranscription = strings.TrimSpace(record[mapping.ExampleWithTranscriptionIndex])
	}

	if mapping.FrequencyIndex >= 0 && mapping.FrequencyIndex < len(record) {
		if freq, err := strconv.Atoi(strings.TrimSpace(record[mapping.FrequencyIndex])); err == nil {
			item.Frequency = freq
		}
	}

	return item
}

// downloadTelegramFile downloads a file from Telegram
func (h *Handler) downloadTelegramFile(fileID string) ([]byte, error) {
	// Get file info from Telegram
	fileURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", h.botToken, fileID)
	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var fileInfo struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&fileInfo); err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to decode file info: %w", err)
	}

	if !fileInfo.OK || fileInfo.Result.FilePath == "" {
		return nil, fmt.Errorf("failed to get file path from Telegram")
	}

	// Download the actual file
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", h.botToken, fileInfo.Result.FilePath)
	fileResp, err := http.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer func() {
		_ = fileResp.Body.Close()
	}()

	// Read file content
	content, err := io.ReadAll(fileResp.Body)
	if err != nil {
		fileResp.Body.Close()
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	return content, nil
}

// Helper functions for sending notifications

func (h *Handler) updateImportStatus(chatID int64, messageID int, status string) {
	editMsg := &telegram.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      status,
		ParseMode: models.ParseModeMarkdown,
	}

	if _, err := h.bot.EditMessageText(context.Background(), editMsg); err != nil {
		log.Printf("Failed to update import status: %v", err)
	}
}

func (h *Handler) sendFileImportSuccess(chatID int64, messageID int, count int, deckID string) {
	// Delete the status message
	deleteMsg := &telegram.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	}
	if _, err := h.bot.DeleteMessage(context.Background(), deleteMsg); err != nil {
		log.Printf("Failed to delete status message: %v", err)
	}

	// Send success message
	msg := &telegram.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("✅ Импорт завершен\\!\n\nИмпортировано карточек: *%d*", count),
		ParseMode: models.ParseModeMarkdown,
		ReplyMarkup: models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{
						Text:   "Открыть колоду",
						WebApp: &models.WebAppInfo{URL: fmt.Sprintf("%s/?deck=%s", h.webAppURL, deckID)},
					},
				},
			},
		},
	}

	if _, err := h.bot.SendMessage(context.Background(), msg); err != nil {
		log.Printf("Failed to send import success message: %v", err)
	}
}

func (h *Handler) sendFileImportError(chatID int64, errorMsg string, messageID int) {
	// Delete the status message
	deleteMsg := &telegram.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	}
	if _, err := h.bot.DeleteMessage(context.Background(), deleteMsg); err != nil {
		log.Printf("Failed to delete status message: %v", err)
	}

	// Send error message
	msg := &telegram.SendMessageParams{
		ChatID:    chatID,
		Text:      fmt.Sprintf("❌ %s", errorMsg),
		ParseMode: models.ParseModeMarkdown,
	}

	if _, err := h.bot.SendMessage(context.Background(), msg); err != nil {
		log.Printf("Failed to send import error message: %v", err)
	}
}
