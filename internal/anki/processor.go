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
	"sync"
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

func (p *Processor) GetMediaFiles(ankiExport *Export, tempDir string) ([]MediaFile, []string) {
	var mediaFiles []MediaFile
	var missingFiles []string

	mediaDir := filepath.Join(tempDir, "media")

	if _, err := os.Stat(mediaDir); os.IsNotExist(err) {

		entries, err := os.ReadDir(tempDir)
		if err != nil {
			return mediaFiles, []string{fmt.Sprintf("failed to read temp directory: %v", err)}
		}

		mediaDirFound := false
		for _, entry := range entries {
			if entry.IsDir() && entry.Name() != "__MACOSX" {
				nestedMediaDir := filepath.Join(tempDir, entry.Name(), "media")
				if _, err := os.Stat(nestedMediaDir); err == nil {
					mediaDir = nestedMediaDir
					mediaDirFound = true
					break
				}
			}
		}

		if !mediaDirFound && len(ankiExport.MediaFiles) > 0 {
			return mediaFiles, []string{fmt.Sprintf("media directory not found in export at %s", tempDir)}
		}
	}

	for _, fileName := range ankiExport.MediaFiles {
		filePath := filepath.Join(mediaDir, fileName)

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			missingFiles = append(missingFiles, fmt.Sprintf("media file not found: %s", filePath))
			continue
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

	return mediaFiles, missingFiles
}

func (p *Processor) UploadMediaFiles(ctx context.Context, mediaFiles []MediaFile) (map[string]string, error) {
	var mu sync.Mutex
	mediaURLs := make(map[string]string)
	var uploadErrors []string

	workerCount := 5
	if len(mediaFiles) < workerCount {
		workerCount = len(mediaFiles)
	}

	if workers, exists := ctx.Value("uploadWorkers").(int); exists && workers > 0 {
		workerCount = workers
		if len(mediaFiles) < workerCount {
			workerCount = len(mediaFiles)
		}
	}

	type uploadResult struct {
		fileName string
		url      string
		err      string
	}

	taskCh := make(chan MediaFile, len(mediaFiles))
	resultCh := make(chan uploadResult, len(mediaFiles))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for media := range taskCh {

				if ctx.Err() != nil {
					return
				}

				result := uploadResult{fileName: media.FileName}

				if _, err := os.Stat(media.FilePath); os.IsNotExist(err) {
					result.err = fmt.Sprintf("media file not found: %s", media.FilePath)
					resultCh <- result
					continue
				}

				file, err := os.Open(media.FilePath)
				if err != nil {
					result.err = fmt.Sprintf("failed to open media file %s: %v", media.FileName, err)
					resultCh <- result
					continue
				}

				timestamp := time.Now().UnixNano()
				uniqueFilename := fmt.Sprintf("anki_import/%d_%s", timestamp, media.FileName)

				url, err := p.storage.UploadFile(ctx, file, uniqueFilename, media.ContentType)
				file.Close()
				if err != nil {
					result.err = fmt.Sprintf("failed to upload media file %s: %v", media.FileName, err)
					resultCh <- result
					continue
				}

				result.url = url
				resultCh <- result
			}
		}()
	}

	go func() {
		for _, media := range mediaFiles {
			select {
			case taskCh <- media:
			case <-ctx.Done():
				return
			}
		}
		close(taskCh)
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for result := range resultCh {
		if result.err != "" {
			mu.Lock()
			uploadErrors = append(uploadErrors, result.err)
			mu.Unlock()
		} else {
			mu.Lock()
			mediaURLs[result.fileName] = result.url
			mu.Unlock()
		}
	}

	if len(uploadErrors) > 0 {
		return mediaURLs, fmt.Errorf("errors uploading media files: %s", strings.Join(uploadErrors, "; "))
	}

	return mediaURLs, nil
}

func (p *Processor) detectLanguageFromNotes(notes []Note, fieldMapping map[string]int) string {

	defaultLanguage := "ja"

	sampleSize := 5
	if len(notes) < sampleSize {
		sampleSize = len(notes)
	}

	for i := 0; i < sampleSize; i++ {
		if i >= len(notes) {
			break
		}

		wordField := ""
		if idx, ok := fieldMapping["Word"]; ok && idx < len(notes[i].Fields) {
			wordField = notes[i].Fields[idx]
		}

		if wordField == "" {
			continue
		}

		japanesePattern := regexp.MustCompile(`[\p{Hiragana}\p{Katakana}\p{Han}]`)
		if japanesePattern.MatchString(wordField) {
			return "ja"
		}

		chinesePattern := regexp.MustCompile(`[\p{Han}]`)
		if chinesePattern.MatchString(wordField) && !japanesePattern.MatchString(wordField) {
			return "zh"
		}

		thaiPattern := regexp.MustCompile(`[\u0E00-\u0E7F]`)
		if thaiPattern.MatchString(wordField) {
			return "th"
		}

		georgianPattern := regexp.MustCompile(`[\u10A0-\u10FF]`)
		if georgianPattern.MatchString(wordField) {
			return "ka"
		}
	}

	return defaultLanguage
}

func (p *Processor) inferTranscriptionType(language string, fieldMapping map[string]int) string {
	switch language {
	case "ja":
		if _, ok := fieldMapping["Word Furigana"]; ok {
			return "furigana"
		}
	case "zh":
		if _, ok := fieldMapping["Word Pinyin"]; ok {
			return "pinyin"
		}
	case "th":
		if _, ok := fieldMapping["Word Romanization"]; ok {
			return "thai_romanization"
		}
	case "ka":
		if _, ok := fieldMapping["Word Transliteration"]; ok {
			return "mkhedruli"
		}
	}

	switch language {
	case "ja":
		return "furigana"
	case "zh":
		return "pinyin"
	case "th":
		return "thai_romanization"
	case "ka":
		return "mkhedruli"
	default:
		return "none"
	}
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

	language := p.detectLanguageFromNotes(ankiExport.Notes, fieldMapping)
	transcriptionType := p.inferTranscriptionType(language, fieldMapping)

	for _, note := range ankiExport.Notes {
		if len(note.Fields) < 5 {
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

		termField := "Word"
		transcriptionField := "Word Reading"
		termWithTranscriptionField := "Word Furigana"
		exampleNativeField := "Sentence"
		exampleWithTranscriptionField := "Sentence Furigana"

		switch language {
		case "zh":
			transcriptionField = "Word Pinyin"
			termWithTranscriptionField = "Word With Pinyin"
			exampleWithTranscriptionField = "Sentence With Pinyin"
		case "th":
			transcriptionField = "Word Romanization"
			termWithTranscriptionField = "Word With Romanization"
			exampleWithTranscriptionField = "Sentence With Romanization"
		case "ka":
			transcriptionField = "Word Transliteration"
			termWithTranscriptionField = "Word With Transliteration"
			exampleWithTranscriptionField = "Sentence With Transliteration"
		}

		vocabItem := db.VocabularyItem{

			Term:                     getFieldValue(note.Fields, fieldMapping, termField, ""),
			Transcription:            getFieldValue(note.Fields, fieldMapping, transcriptionField, ""),
			TermWithTranscription:    getFieldValue(note.Fields, fieldMapping, termWithTranscriptionField, ""),
			MeaningEn:                getFieldValue(note.Fields, fieldMapping, "Word Meaning", ""),
			MeaningRu:                getFieldValue(note.Fields, fieldMapping, "Word Meaning Russian", ""),
			ExampleNative:            getFieldValue(note.Fields, fieldMapping, exampleNativeField, ""),
			ExampleWithTranscription: getFieldValue(note.Fields, fieldMapping, exampleWithTranscriptionField, ""),
			ExampleEn:                getFieldValue(note.Fields, fieldMapping, "Sentence Meaning", ""),
			ExampleRu:                getFieldValue(note.Fields, fieldMapping, "Sentence Meaning Russian", ""),
			LanguageCode:             language,
			TranscriptionType:        transcriptionType,
			AudioWord:                wordAudioURL,
			AudioExample:             exampleAudioURL,
			ImageURL:                 imageURL,
		}

		vocabItems = append(vocabItems, vocabItem)
	}

	return vocabItems, nil
}

func (p *Processor) ImportDeck(ctx context.Context, userID, deckName, zipFilePath, languageCode string) (*ImportResult, error) {
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

	result.LanguageCode = languageCode

	mediaFiles, mediaErrors := p.GetMediaFiles(ankiExport, tempDir)

	for _, errMsg := range mediaErrors {
		result.Errors = append(result.Errors, fmt.Sprintf("error getting media files: %v", errMsg))
	}

	mediaURLs := make(map[string]string)
	if len(mediaFiles) > 0 {
		mediaURLs, err = p.UploadMediaFiles(ctx, mediaFiles)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("error uploading media files: %v", err))
		}
	}
	result.MediaUploaded = len(mediaURLs)

	vocabItems, err := p.ConvertToVocabularyItems(ankiExport, mediaURLs)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("error converting notes: %v", err))
	}

	if languageCode == "" {
		languageCode = "ja"
	}

	transcriptionType := p.inferTranscriptionType(languageCode, map[string]int{})

	result.TranscriptionType = transcriptionType

	for i := range vocabItems {
		vocabItems[i].LanguageCode = languageCode
		vocabItems[i].TranscriptionType = transcriptionType
	}

	deck, err := p.db.CreateDeck(userID, result.DeckName, fmt.Sprintf("Imported from Anki: %s", result.DeckName), "", languageCode, transcriptionType)
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
