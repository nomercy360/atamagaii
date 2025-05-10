package anki

import (
	"archive/zip"
	"atamagaii/internal/db"
	"atamagaii/internal/storage"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Processor struct {
	db          *db.Storage
	storage     storage.Provider
	tempDirBase string
}

func NewProcessor(dbStorage *db.Storage, storageProvider storage.Provider) *Processor {
	return &Processor{
		db:          dbStorage,
		storage:     storageProvider,
		tempDirBase: os.TempDir(),
	}
}

func (p *Processor) ExtractExport(zipFile string) (*Export, string, error) {

	tempDir, err := os.MkdirTemp(p.tempDirBase, "anki_export_")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return nil, tempDir, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(tempDir, file.Name)

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
			return nil, tempDir, fmt.Errorf("failed to create directory: %w", err)
		}

		outFile, err := os.Create(path)
		if err != nil {
			return nil, tempDir, fmt.Errorf("failed to create file: %w", err)
		}

		inFile, err := file.Open()
		if err != nil {
			outFile.Close()
			return nil, tempDir, fmt.Errorf("failed to open compressed file: %w", err)
		}

		_, err = io.Copy(outFile, inFile)
		outFile.Close()
		inFile.Close()
		if err != nil {
			return nil, tempDir, fmt.Errorf("failed to copy file contents: %w", err)
		}
	}

	deckPath := filepath.Join(tempDir, "deck.json")

	if _, err := os.Stat(deckPath); os.IsNotExist(err) {

		entries, err := os.ReadDir(tempDir)
		if err != nil {
			return nil, tempDir, fmt.Errorf("failed to read temp directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() && entry.Name() != "__MACOSX" {
				nestedPath := filepath.Join(tempDir, entry.Name(), "deck.json")
				if _, err := os.Stat(nestedPath); err == nil {
					deckPath = nestedPath
					break
				}
			}
		}
	}

	deckFile, err := os.Open(deckPath)
	if err != nil {
		return nil, tempDir, fmt.Errorf("failed to open deck.json: %w", err)
	}
	defer deckFile.Close()

	var ankiExport Export
	decoder := json.NewDecoder(deckFile)
	if err := decoder.Decode(&ankiExport); err != nil {
		return nil, tempDir, fmt.Errorf("failed to parse deck.json: %w", err)
	}

	return &ankiExport, tempDir, nil
}

func (p *Processor) GetMediaFiles(ankiExport *Export, tempDir string) ([]MediaFile, error) {
	var mediaFiles []MediaFile

	mediaDir := filepath.Join(tempDir, "media")

	if _, err := os.Stat(mediaDir); os.IsNotExist(err) {
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read temp directory: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() && entry.Name() != "__MACOSX" {
				nestedMediaDir := filepath.Join(tempDir, entry.Name(), "media")
				if _, err := os.Stat(nestedMediaDir); err == nil {
					mediaDir = nestedMediaDir
					break
				}
			}
		}
	}

	for _, fileName := range ankiExport.MediaFiles {
		filePath := filepath.Join(mediaDir, fileName)

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("media file not found: %s", filePath)
		}

		contentType := ""
		if strings.HasSuffix(fileName, ".mp3") {
			contentType = "audio/mpeg"
		} else if strings.HasSuffix(fileName, ".jpg") || strings.HasSuffix(fileName, ".jpeg") {
			contentType = "image/jpeg"
		} else if strings.HasSuffix(fileName, ".png") {
			contentType = "image/png"
		} else {

			contentType = "application/octet-stream"
		}

		mediaFiles = append(mediaFiles, MediaFile{
			FileName:    fileName,
			ContentType: contentType,
			FilePath:    filePath,
		})
	}

	return mediaFiles, nil
}

func (p *Processor) UploadMediaFiles(ctx context.Context, mediaFiles []MediaFile) (map[string]string, error) {
	mediaURLs := make(map[string]string)
	var uploadErrors []string

	for _, media := range mediaFiles {

		if _, err := os.Stat(media.FilePath); os.IsNotExist(err) {
			uploadErrors = append(uploadErrors, fmt.Sprintf("media file not found: %s", media.FilePath))
			continue
		}

		file, err := os.Open(media.FilePath)
		if err != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("failed to open media file %s: %v", media.FileName, err))
			continue
		}

		timestamp := time.Now().UnixNano()
		uniqueFilename := fmt.Sprintf("anki_import/%d_%s", timestamp, media.FileName)

		url, err := p.storage.UploadFile(ctx, file, uniqueFilename, media.ContentType)
		file.Close()
		if err != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("failed to upload media file %s: %v", media.FileName, err))
			continue
		}

		mediaURLs[media.FileName] = url
	}

	if len(uploadErrors) > 0 {
		return mediaURLs, fmt.Errorf("errors uploading media files: %s", strings.Join(uploadErrors, "; "))
	}

	return mediaURLs, nil
}

func (p *Processor) ConvertToVocabularyItems(ankiExport *Export, mediaURLs map[string]string) ([]db.VocabularyItem, error) {
	var vocabItems []db.VocabularyItem

	if len(ankiExport.NoteModels) == 0 {
		return nil, fmt.Errorf("no note models found in export")
	}

	fieldMapping := make(map[string]int)
	for _, field := range ankiExport.NoteModels[0].Fields {
		fieldMapping[field.Name] = field.Ord
	}

	for _, note := range ankiExport.Notes {

		if len(note.Fields) < 10 {
			continue
		}

		imageURL := ""
		if picFieldIdx, ok := fieldMapping["Picture"]; ok && picFieldIdx < len(note.Fields) {
			imgSrc := note.Fields[picFieldIdx]
			imgRegex := regexp.MustCompile(`<img src="([^"]+)"`)
			if matches := imgRegex.FindStringSubmatch(imgSrc); len(matches) > 1 {
				fileName := matches[1]
				if url, ok := mediaURLs[fileName]; ok {
					imageURL = url
				}
			}
		}

		wordAudioURL := ""
		if wordAudioIdx, ok := fieldMapping["Word Audio"]; ok && wordAudioIdx < len(note.Fields) {
			audioSrc := note.Fields[wordAudioIdx]
			audioRegex := regexp.MustCompile(`\[sound:([^]]+)]`)
			if matches := audioRegex.FindStringSubmatch(audioSrc); len(matches) > 1 {
				fileName := matches[1]
				if url, ok := mediaURLs[fileName]; ok {
					wordAudioURL = url
				}
			}
		}

		exampleAudioURL := ""
		if exampleAudioIdx, ok := fieldMapping["Sentence Audio"]; ok && exampleAudioIdx < len(note.Fields) {
			audioSrc := note.Fields[exampleAudioIdx]
			audioRegex := regexp.MustCompile(`\[sound:([^]]+)]`)
			if matches := audioRegex.FindStringSubmatch(audioSrc); len(matches) > 1 {
				fileName := matches[1]
				if url, ok := mediaURLs[fileName]; ok {
					exampleAudioURL = url
				}
			}
		}

		vocabItem := db.VocabularyItem{
			Word:            getFieldValue(note.Fields, fieldMapping, "Word", ""),
			Reading:         getFieldValue(note.Fields, fieldMapping, "Word Reading", ""),
			WordFurigana:    getFieldValue(note.Fields, fieldMapping, "Word Furigana", ""),
			MeaningEn:       getFieldValue(note.Fields, fieldMapping, "Word Meaning", ""),
			MeaningRu:       "",
			ExampleJa:       getFieldValue(note.Fields, fieldMapping, "Sentence", ""),
			ExampleEn:       getFieldValue(note.Fields, fieldMapping, "Sentence Meaning", ""),
			ExampleRu:       "",
			ExampleFurigana: getFieldValue(note.Fields, fieldMapping, "Sentence Furigana", ""),
			AudioWord:       wordAudioURL,
			AudioExample:    exampleAudioURL,
			ImageURL:        imageURL,
		}

		vocabItems = append(vocabItems, vocabItem)
	}

	return vocabItems, nil
}

func (p *Processor) ImportDeck(ctx context.Context, userID, deckName, zipFilePath string) (*ImportResult, error) {
	result := &ImportResult{
		Errors: []string{},
	}

	ankiExport, tempDir, err := p.ExtractExport(zipFilePath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("error extracting Anki export: %v", err))
		return result, err
	}
	defer os.RemoveAll(tempDir)

	result.DeckName = ankiExport.Name
	if deckName != "" {
		result.DeckName = deckName
	}

	mediaFiles, err := p.GetMediaFiles(ankiExport, tempDir)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("error getting media files: %v", err))
	}

	mediaURLs, err := p.UploadMediaFiles(ctx, mediaFiles)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("error uploading media files: %v", err))
	}
	result.MediaUploaded = len(mediaURLs)

	vocabItems, err := p.ConvertToVocabularyItems(ankiExport, mediaURLs)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("error converting notes: %v", err))
	}

	deck, err := p.db.CreateDeck(userID, result.DeckName, fmt.Sprintf("Imported from Anki: %s", result.DeckName), "")
	if err != nil {
		return result, fmt.Errorf("failed to create deck: %w", err)
	}

	var fieldsArray []string
	for _, vocabItem := range vocabItems {
		vocabJSON, err := json.Marshal(vocabItem)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("error marshaling vocabulary item: %v", err))
			continue
		}

		fieldsArray = append(fieldsArray, string(vocabJSON))
	}

	if len(fieldsArray) > 0 {
		err = p.db.AddCardsInBatch(userID, deck.ID, fieldsArray)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("error adding cards: %v", err))
		}
	}

	result.CardsAdded = len(fieldsArray)
	return result, nil
}

func getFieldValue(fields []string, mapping map[string]int, fieldName, defaultValue string) string {
	if idx, ok := mapping[fieldName]; ok && idx < len(fields) {
		return fields[idx]
	}
	return defaultValue
}
