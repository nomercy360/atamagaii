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

	// Template for vocabulary recall tasks
	TaskVocabRecallTemplate = "task_vocab_recall_reverse.json"
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
		// Generate task for this card
		taskContent, err := tg.openAIClient.GenerateTask(ctx, &card, TaskVocabRecallTemplate)
		if err != nil {
			log.Printf("Error generating task for card %s: %v", card.ID, err)
			continue
		}

		// Convert task content to JSON
		contentJSON, err := json.Marshal(taskContent)
		if err != nil {
			log.Printf("Error marshaling task content for card %s: %v", card.ID, err)
			continue
		}

		// Store task in database
		correctAnswer := ""
		if taskContent.CorrectAnswer != "" {
			// Store the correct answer option text
			correctAnswer = taskContent.Options[taskContent.CorrectAnswer]
		}

		_, err = tg.storage.AddTask("vocab_recall_reverse", string(contentJSON), correctAnswer, card.ID, card.UserID)
		if err != nil {
			log.Printf("Error saving task for card %s: %v", card.ID, err)
			continue
		}

		log.Printf("Successfully generated task for card %s (user %s)", card.ID, card.UserID)
	}

	log.Println("Task generation job completed")
}
