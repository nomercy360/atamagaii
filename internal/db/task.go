package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	nanoid "github.com/matoous/go-nanoid/v2"
	"time"
)

// Task represents a task in the database
type Task struct {
	ID           string     `db:"id" json:"id"`
	Type         string     `db:"type" json:"type"`
	Content      string     `db:"content" json:"content"`
	Answer       string     `db:"answer" json:"answer"`
	CardID       string     `db:"card_id" json:"card_id"`
	UserID       string     `db:"user_id" json:"user_id"`
	CompletedAt  *time.Time `db:"completed_at" json:"completed_at,omitempty"`
	UserResponse *string    `db:"user_response" json:"user_response,omitempty"`
	IsCorrect    *bool      `db:"is_correct" json:"is_correct,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt    *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// TaskContent represents the content of a task
type TaskContent struct {
	Question      string            `json:"question"`
	Options       map[string]string `json:"options"`
	CorrectAnswer string            `json:"correct_answer"`
}

// AddTask adds a new task to the database
func (s *Storage) AddTask(taskType, content, answer, cardID, userID string) (*Task, error) {
	taskID := nanoid.Must()
	now := time.Now()

	query := `
		INSERT INTO tasks (
			id, type, content, answer, card_id, user_id, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(
		query,
		taskID,
		taskType,
		content,
		answer,
		cardID,
		userID,
		now,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("error adding task: %w", err)
	}

	return &Task{
		ID:        taskID,
		Type:      taskType,
		Content:   content,
		Answer:    answer,
		CardID:    cardID,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetTask gets a task by ID
func (s *Storage) GetTask(taskID string) (*Task, error) {
	query := `
		SELECT id, type, content, answer, card_id, user_id, created_at, updated_at, deleted_at
		FROM tasks
		WHERE id = ? AND deleted_at IS NULL
	`

	var task Task
	err := s.db.QueryRow(query, taskID).Scan(
		&task.ID,
		&task.Type,
		&task.Content,
		&task.Answer,
		&task.CardID,
		&task.UserID,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("error getting task: %w", err)
	}

	return &task, nil
}

// GetCardsForTaskGeneration retrieves cards that have moved to review state today and need tasks generated
func (s *Storage) GetCardsForTaskGeneration() ([]Card, error) {
	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	// Get cards that were reviewed today, are in review state, and have next_review in the future
	// Use LEFT JOIN to exclude cards that already have tasks generated today
	query := `
		SELECT c.id, c.deck_id, c.fields, c.user_id, c.next_review, c.interval, c.ease,
		       c.review_count, c.laps_count, c.last_reviewed_at, c.first_reviewed_at, c.state,
		       c.learning_step, c.created_at, c.updated_at, c.deleted_at
		FROM cards c
		LEFT JOIN (
			SELECT DISTINCT card_id, user_id
			FROM tasks
			WHERE created_at >= ?
			AND created_at < ?
			AND deleted_at IS NULL
		) t ON c.id = t.card_id AND c.user_id = t.user_id
		WHERE c.state = ?
		  AND c.deleted_at IS NULL
		  AND c.last_reviewed_at >= ?
		  AND c.last_reviewed_at < ?
		  AND c.next_review > ?
		  AND t.card_id IS NULL
	`

	rows, err := s.db.Query(query, today, tomorrow, StateReview, today, tomorrow, time.Now())
	if err != nil {
		return nil, fmt.Errorf("error getting cards for task generation: %w", err)
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		var card Card
		var intervalNs int64

		if err := rows.Scan(
			&card.ID,
			&card.DeckID,
			&card.Fields,
			&card.UserID,
			&card.NextReview,
			&intervalNs,
			&card.Ease,
			&card.ReviewCount,
			&card.LapsCount,
			&card.LastReviewedAt,
			&card.FirstReviewedAt,
			&card.State,
			&card.LearningStep,
			&card.CreatedAt,
			&card.UpdatedAt,
			&card.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning card for task generation: %w", err)
		}

		card.Interval = time.Duration(intervalNs)
		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating cards for task generation: %w", err)
	}

	return cards, nil
}

func (s *Storage) GetTasksDueForUser(userID string, limit int, deckID string) ([]Task, error) {
	query := `
		SELECT t.id, t.type, t.content, t.answer, t.card_id, t.user_id, 
		       t.completed_at, t.user_response, t.is_correct, 
		       t.created_at, t.updated_at, t.deleted_at
		FROM tasks t
		JOIN cards c ON t.card_id = c.id AND t.user_id = c.user_id
		WHERE t.user_id = ?
		  AND t.deleted_at IS NULL
		  AND t.completed_at IS NULL
		  AND c.state = ?
	`

	args := []interface{}{userID, StateReview}

	if deckID != "" {
		query += " AND c.deck_id = ?"
		args = append(args, deckID)
	}

	query += " ORDER BY t.created_at LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error getting due tasks for user: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		if err := rows.Scan(
			&task.ID,
			&task.Type,
			&task.Content,
			&task.Answer,
			&task.CardID,
			&task.UserID,
			&task.CompletedAt,
			&task.UserResponse,
			&task.IsCorrect,
			&task.CreatedAt,
			&task.UpdatedAt,
			&task.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning due task: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating due task rows: %w", err)
	}

	return tasks, nil
}

// TasksPerDeck represents a summary of tasks for a specific deck
type TasksPerDeck struct {
	DeckID     string `json:"deck_id"`
	DeckName   string `json:"deck_name"`
	TotalTasks int    `json:"total_tasks"`
}

// GetTaskStatsByDeck returns a summary of available tasks per deck for a user
func (s *Storage) GetTaskStatsByDeck(userID string) ([]TasksPerDeck, error) {
	query := `
		SELECT d.id, d.name, COUNT(t.id) as task_count
		FROM decks d
		LEFT JOIN cards c ON d.id = c.deck_id
		LEFT JOIN tasks t ON c.id = t.card_id
		WHERE d.user_id = ?
		  AND d.deleted_at IS NULL
		  AND (t.user_id = ? OR t.user_id IS NULL)
		  AND (t.deleted_at IS NULL OR t.deleted_at IS NULL)
		  AND (t.completed_at IS NULL OR t.completed_at IS NULL)
		GROUP BY d.id, d.name
		HAVING COUNT(t.id) > 0
		ORDER BY d.name
	`

	rows, err := s.db.Query(query, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("error getting task stats by deck: %w", err)
	}
	defer rows.Close()

	var stats []TasksPerDeck
	for rows.Next() {
		var stat TasksPerDeck
		if err := rows.Scan(&stat.DeckID, &stat.DeckName, &stat.TotalTasks); err != nil {
			return nil, fmt.Errorf("error scanning task stats: %w", err)
		}
		stats = append(stats, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating task stats rows: %w", err)
	}

	return stats, nil
}

// UnmarshalTaskContent unmarshals the task content string into a TaskContent struct
func (t *Task) UnmarshalTaskContent() (interface{}, error) {
	if t.Type == "vocab_recall_reverse" {
		var content TaskContent
		err := json.Unmarshal([]byte(t.Content), &content)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling vocab recall task content: %w", err)
		}
		return &content, nil
	} else if t.Type == "sentence_translation" {
		// For sentence translation tasks
		var content struct {
			SentenceRu     string `json:"sentence_ru"`
			SentenceNative string `json:"sentence_native"`
		}
		err := json.Unmarshal([]byte(t.Content), &content)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling sentence translation task content: %w", err)
		}
		return &content, nil
	}

	// Default fallback
	var content map[string]interface{}
	err := json.Unmarshal([]byte(t.Content), &content)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling task content: %w", err)
	}
	return content, nil
}

// SubmitTaskResponse submits a user's response to a task and marks it as completed
func (s *Storage) SubmitTaskResponse(taskID, userID, response string) (*Task, error) {
	// First get the task to verify it exists and belongs to the user
	task, err := s.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving task: %w", err)
	}

	if task.UserID != userID {
		return nil, fmt.Errorf("task does not belong to user")
	}

	if task.CompletedAt != nil {
		return nil, fmt.Errorf("task is already completed")
	}

	// Check if the response is correct
	isCorrect := false

	if task.Type == "vocab_recall_reverse" {
		// For vocab recall tasks, we now store only the letter (A, B, C, or D)
		// and the user should respond with that letter
		isCorrect = task.Answer == response
	} else if task.Type == "sentence_translation" {
		// For sentence translation tasks, we'll initially use exact matching
		// This could be enhanced with fuzzy matching or AI verification later
		isCorrect = task.Answer == response
	} else {
		// Default fallback for other task types
		isCorrect = task.Answer == response
	}

	// Update the task as completed
	now := time.Now()
	query := `
		UPDATE tasks
		SET completed_at = ?,
		    user_response = ?,
		    is_correct = ?,
		    updated_at = ?
		WHERE id = ? AND user_id = ? AND deleted_at IS NULL
	`

	result, err := s.db.Exec(query, now, response, isCorrect, now, taskID, userID)
	if err != nil {
		return nil, fmt.Errorf("error updating task completion: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error checking task update: %w", err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("no task updated")
	}

	// Return the updated task
	task.CompletedAt = &now
	task.UserResponse = &response
	task.IsCorrect = &isCorrect
	task.UpdatedAt = now

	return task, nil
}
