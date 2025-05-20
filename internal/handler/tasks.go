package handler

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
)

func (h *Handler) GetTasks(c echo.Context) error {
	userID, _ := GetUserIDFromToken(c)
	limitParam := c.QueryParam("limit")
	deckID := c.QueryParam("deck_id")

	limit := 10
	if limitParam != "" {
		parsedLimit, err := strconv.Atoi(limitParam)
		if err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	tasks, err := h.db.GetTasksDueForUser(userID, limit, deckID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error retrieving tasks: %v", err))
	}

	var taskResponses []contract.TaskResponse
	for _, task := range tasks {
		var content contract.TaskContent

		switch task.Type {
		case db.TaskTypeVocabRecall:
			content, err = db.UnmarshalTaskContent[db.TaskVocabRecallContent](&task)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error parsing vocab recall task content: %v", err))
			}
		case db.TaskTypeSentenceTranslation:
			content, err = db.UnmarshalTaskContent[db.TaskSentenceTranslationContent](&task)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error parsing sentence translation task content: %v", err))
			}
		case db.TaskTypeAudio:
			content, err = db.UnmarshalTaskContent[db.TaskAudioContent](&task)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error parsing audio task content: %v", err))
			}
		default:
			return echo.NewHTTPError(http.StatusNotImplemented, "Task type not implemented")
		}

		taskResponse := contract.TaskResponse{
			ID:           task.ID,
			Type:         string(task.Type),
			Content:      content,
			CompletedAt:  task.CompletedAt,
			UserResponse: task.UserResponse,
			IsCorrect:    task.IsCorrect,
			CreatedAt:    task.CreatedAt,
		}
		taskResponses = append(taskResponses, taskResponse)
	}

	return c.JSON(http.StatusOK, taskResponses)
}

// GetTasksPerDeck returns a list of tasks grouped by deck
func (h *Handler) GetTasksPerDeck(c echo.Context) error {
	userID, _ := GetUserIDFromToken(c)

	tasksPerDeck, err := h.db.GetTaskStatsByDeck(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error retrieving tasks stats: %v", err))
	}

	return c.JSON(http.StatusOK, tasksPerDeck)
}

// SubmitTaskResponse handles the POST /api/tasks/submit endpoint to submit a task response
func (h *Handler) SubmitTaskResponse(c echo.Context) error {
	userID, _ := GetUserIDFromToken(c)

	var req contract.SubmitTaskRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
	}

	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Validation error: %v", err))
	}

	task, err := h.db.GetTask(req.TaskID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error retrieving task: %v", err))
	}

	if task.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "You are not allowed to submit this task")
	}

	if task.CompletedAt != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Task already completed")
	}

	isCorrect := false
	var feedback *string

	if task.Type == db.TaskTypeVocabRecall {
		isCorrect = req.Response == task.Answer
	} else if task.Type == db.TaskTypeSentenceTranslation {
		translationContent, err := db.UnmarshalTaskContent[db.TaskSentenceTranslationContent](task)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error parsing translation task content: %v", err))
		}

		ctx := c.Request().Context()
		checkResult, err := h.openaiClient.CheckSentenceTranslation(
			ctx,
			translationContent.SentenceRu,
			task.Answer,  // Correct answer from the DB
			req.Response, // User-provided answer
			"jp",         // TODO: Get the language code from the card
		)

		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error checking translation: %v", err))
		}

		// Update the is_correct field based on the AI check (score >= 80 is considered correct)
		isCorrect = checkResult.Score >= 80
		if checkResult.Feedback != nil && !isCorrect {
			feedback = checkResult.Feedback
		}
	} else if task.Type == db.TaskTypeAudio {
		// For audio tasks, just check if the user selected the correct option
		isCorrect = req.Response == task.Answer
	} else {
		return echo.NewHTTPError(http.StatusNotImplemented, "Task type not implemented")
	}

	if err := h.db.SubmitTaskResponse(req.TaskID, userID, req.Response, isCorrect); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error submitting task response: %v", err))
	}

	response := contract.SubmitTaskResponse{
		IsCorrect: isCorrect,
		FeedBack:  feedback,
	}

	return c.JSON(http.StatusOK, response)
}
