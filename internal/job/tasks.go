package job

import (
	"atamagaii/internal/ai"
	"atamagaii/internal/db"
	"context"
	"encoding/json"
	"log"
	"time"
)

const (
	// TaskGenInterval is how often the task generation job runs
	TaskGenInterval = 2 * time.Minute

	// Task type constants
	TaskTypeVocabRecall         = "vocab_recall_reverse"
	TaskTypeSentenceTranslation = "sentence_translation"

	// Template files
	TaskVocabRecallTemplate         = "task_vocab_recall_reverse.json"
	TaskSentenceTranslationTemplate = "task_sentence_translation.json"
)

// TaskGenerator is responsible for generating tasks for cards in review state
type TaskGenerator struct {
	storage      *db.Storage
	openAIClient *ai.OpenAIClient
	stopCh       chan struct{}
	runningLock  chan struct{} // Used to ensure only one task generation job runs at a time
}

// NewTaskGenerator creates a new TaskGenerator
func NewTaskGenerator(storage *db.Storage, openAIClient *ai.OpenAIClient) *TaskGenerator {
	return &TaskGenerator{
		storage:      storage,
		openAIClient: openAIClient,
		stopCh:       make(chan struct{}),
		runningLock:  make(chan struct{}, 1), // Buffer of 1 allows us to use it as a semaphore
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
		// For now, we'll use a 50/50 split between vocab recall and sentence translation
		// This can be adjusted later based on user preferences or card type
		taskType := TaskTypeVocabRecall
		templateName := TaskVocabRecallTemplate

		// Simple random selection - can be made more sophisticated later
		if time.Now().Unix()%2 == 0 {
			taskType = TaskTypeSentenceTranslation
			templateName = TaskSentenceTranslationTemplate
		}

		// Generate task for this card
		taskContent, err := tg.openAIClient.GenerateTask(ctx, &card, templateName)
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
		if taskType == TaskTypeVocabRecall {
			// For vocab recall tasks, extract and store just the answer letter
			var vocabContent struct {
				CorrectAnswer string `json:"correct_answer"`
				Options       struct {
					A string `json:"a"`
					B string `json:"b"`
					C string `json:"c"`
					D string `json:"d"`
				} `json:"options"`
				Question string `json:"question"`
			}

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

		} else if taskType == TaskTypeSentenceTranslation {
			// For sentence translation tasks, extract the native sentence as correct answer
			var translationContent struct {
				SentenceRu     string `json:"sentence_ru"`
				SentenceNative string `json:"sentence_native"`
			}

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
