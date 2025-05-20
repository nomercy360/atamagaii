package job

import (
	"atamagaii/internal/ai"
	"atamagaii/internal/db"
	"atamagaii/internal/storage"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

const (
	// TaskGenInterval is how often the task generation job runs
	TaskGenInterval                 = 2 * time.Minute
	TaskVocabRecallTemplate         = "task_vocab_recall.json"
	TaskSentenceTranslationTemplate = "task_sentence_translation.json"
	TaskAudioTemplate               = "task_audio.json"
)

// TaskGenerator is responsible for generating tasks for cards in review state
type TaskGenerator struct {
	storage         *db.Storage
	openAIClient    *ai.OpenAIClient
	storageProvider storage.Provider
	stopCh          chan struct{}
	runningLock     chan struct{} // Used to ensure only one task generation job runs at a time
}

// NewTaskGenerator creates a new TaskGenerator
func NewTaskGenerator(storage *db.Storage, openAIClient *ai.OpenAIClient, storageProvider storage.Provider) *TaskGenerator {
	return &TaskGenerator{
		storage:         storage,
		openAIClient:    openAIClient,
		storageProvider: storageProvider,
		stopCh:          make(chan struct{}),
		runningLock:     make(chan struct{}, 1), // Buffer of 1 allows us to use it as a semaphore
	}
}

// Start begins the task generation job
func (tg *TaskGenerator) Start() {
	log.Println("Starting task generation job")

	ticker := time.NewTicker(TaskGenInterval)
	defer ticker.Stop()

	// Run once immediately
	go tg.generateTasks()

	for {
		select {
		case <-ticker.C:
			go tg.generateTasks()
		case <-tg.stopCh:
			log.Println("Task generation job stopped")
			return
		}
	}
}

// Stop stops the task generation job
func (tg *TaskGenerator) Stop() {
	close(tg.stopCh)
}

// generateTasks finds cards in review state that need tasks and generates them
func (tg *TaskGenerator) generateTasks() {
	// Use non-blocking send to check if another job is already running
	select {
	case tg.runningLock <- struct{}{}: // Acquired the lock
		defer func() { <-tg.runningLock }() // Release the lock when we're done
	default:
		// Another job is already running, exit without doing anything
		log.Println("Task generation job already running, skipping this execution")
		return
	}

	log.Println("Running task generation job")

	// Get cards that have moved to review state today and need tasks
	cards, err := tg.storage.GetCardsForTaskGeneration()
	if err != nil {
		log.Printf("Error getting cards for task generation: %v", err)
		return
	}

	log.Printf("Found %d cards that need tasks generated", len(cards))

	ctx := context.Background()

	for _, card := range cards {
		// Randomly choose between task types
		// For now, we'll use a random distribution between task types
		// This can be adjusted later based on user preferences or card type
		taskType := db.TaskTypeVocabRecall
		templateName := TaskVocabRecallTemplate

		// Simple random selection - can be made more sophisticated later
		rand := time.Now().Unix() % 3
		if rand == 0 {
			taskType = db.TaskTypeSentenceTranslation
			templateName = TaskSentenceTranslationTemplate
		} else if rand == 1 {
			taskType = db.TaskTypeAudio
			templateName = TaskAudioTemplate
		}

		var vocabItem db.VocabularyItem
		if err := json.Unmarshal([]byte(card.Fields), &vocabItem); err != nil {
			log.Printf("error unmarshaling card fields: %v", err)
			continue
		}

		targetWord := vocabItem.Term
		if vocabItem.MeaningEn != "" {
			targetWord = fmt.Sprintf("%s (%s)", vocabItem.Term, vocabItem.MeaningEn)
		}

		// Generate task for this card
		taskContent, err := tg.openAIClient.GenerateTask(ctx, vocabItem.LanguageCode, templateName, targetWord)
		if err != nil {
			log.Printf("Error generating task for card %s: %v", card.ID, err)
			continue
		}

		// First convert to JSON to work with it
		rawContentJSON, err := json.Marshal(taskContent)
		if err != nil {
			log.Printf("Error marshaling raw task content for card %s: %v", card.ID, err)
			continue
		}

		// Store task in database
		correctAnswer := ""
		var contentJSON []byte

		// Handle different task types
		if taskType == db.TaskTypeVocabRecall {
			// For vocab recall tasks, extract and store just the answer letter
			var vocabContent db.TaskVocabRecallContent

			if err := json.Unmarshal(rawContentJSON, &vocabContent); err != nil {
				log.Printf("Error parsing vocab content for card %s: %v", card.ID, err)
				continue
			}

			// Store just the answer letter (A, B, C, D)
			correctAnswer = vocabContent.CorrectAnswer

			// Create a content version without the correct answer field
			sanitizedContent := struct {
				Options struct {
					A string `json:"a"`
					B string `json:"b"`
					C string `json:"c"`
					D string `json:"d"`
				} `json:"options"`
				Question string `json:"question"`
			}{
				Options:  vocabContent.Options,
				Question: vocabContent.Question,
			}

			// Marshal again without the correct answer
			contentJSON, err = json.Marshal(sanitizedContent)
			if err != nil {
				log.Printf("Error marshaling sanitized vocab content for card %s: %v", card.ID, err)
				continue
			}

		} else if taskType == db.TaskTypeSentenceTranslation {
			// For sentence translation tasks, extract the native sentence as correct answer
			var translationContent db.TaskSentenceTranslationContent

			if err := json.Unmarshal(rawContentJSON, &translationContent); err != nil {
				log.Printf("Error parsing translation content for card %s: %v", card.ID, err)
				continue
			}

			// Store the native sentence as the correct answer
			correctAnswer = translationContent.SentenceNative

			// Create sanitized content with only the Russian sentence
			sanitizedContent := struct {
				SentenceRu string `json:"sentence_ru"`
			}{
				SentenceRu: translationContent.SentenceRu,
			}

			// Marshal again with only the Russian part
			contentJSON, err = json.Marshal(sanitizedContent)
			if err != nil {
				log.Printf("Error marshaling sanitized translation content for card %s: %v", card.ID, err)
				continue
			}
		} else if taskType == db.TaskTypeAudio {
			// For audio listening tasks, extract and store the correct answer
			var content db.TaskAudioContent

			if err := json.Unmarshal(rawContentJSON, &content); err != nil {
				log.Printf("Error parsing audio content for card %s: %v", card.ID, err)
				continue
			}

			// Store just the answer letter (a, b, c, d)
			correctAnswer = content.CorrectAnswer

			// Generate audio for the story text
			tempFilePath, err := tg.openAIClient.GenerateAudio(ctx, content.Story, vocabItem.LanguageCode)
			if err != nil {
				log.Printf("Error generating audio for task card %s: %v", card.ID, err)
				// Continue without audio, we'll just have text
			} else if tempFilePath != "" {
				// Open the temp file
				tempFile, err := os.Open(tempFilePath)
				if err != nil {
					log.Printf("Error opening temp audio file for task card %s: %v", card.ID, err)
				} else {
					defer tempFile.Close()
					defer os.Remove(tempFilePath)

					// Upload to S3
					audioFileName := fmt.Sprintf("tasks/%s_audio.m4a", card.ID)
					audioURL, err := tg.storageProvider.UploadFile(
						ctx,
						tempFile,
						audioFileName,
						"audio/aac",
					)
					if err != nil {
						log.Printf("Error uploading audio for task card %s: %v", card.ID, err)
					} else {
						content.AudioURL = audioURL
					}
				}
			}

			sanitizedContent := struct {
				Story    string `json:"story"`
				Question string `json:"question"`
				Options  struct {
					A string `json:"a"`
					B string `json:"b"`
					C string `json:"c"`
					D string `json:"d"`
				} `json:"options"`
				AudioURL string `json:"audio_url,omitempty"`
			}{
				Options:  content.Options,
				Question: content.Question,
				Story:    content.Story,
				AudioURL: content.AudioURL,
			}

			// Marshal again without the correct answer
			contentJSON, err = json.Marshal(sanitizedContent)
			if err != nil {
				log.Printf("Error marshaling sanitized audio content for card %s: %v", card.ID, err)
				continue
			}
		} else {
			// For any other task types
			contentJSON = rawContentJSON
		}

		// Add task to database
		_, err = tg.storage.AddTask(taskType, string(contentJSON), correctAnswer, card.ID, card.UserID)
		if err != nil {
			log.Printf("Error saving task for card %s: %v", card.ID, err)
			continue
		}

		log.Printf("Successfully generated %s task for card %s (user %s)", taskType, card.ID, card.UserID)
	}

	log.Println("Task generation job completed")
}
