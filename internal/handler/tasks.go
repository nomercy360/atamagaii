package handler

import (
	"atamagaii/internal/contract"
	"encoding/json"
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
		if err := json.Unmarshal([]byte(task.Content), &content); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error parsing task content: %v", err))
		}

		taskResponse := contract.TaskResponse{
			ID:           task.ID,
			Type:         task.Type,
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

	task, err := h.db.SubmitTaskResponse(req.TaskID, userID, req.Response)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error submitting task response: %v", err))
	}

	// Parse the task content
	var content contract.TaskContent
	if err := json.Unmarshal([]byte(task.Content), &content); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Error parsing task content: %v", err))
	}

	taskResponse := contract.TaskResponse{
		ID:           task.ID,
		Type:         task.Type,
		Content:      content,
		CompletedAt:  task.CompletedAt,
		UserResponse: task.UserResponse,
		IsCorrect:    task.IsCorrect,
		CreatedAt:    task.CreatedAt,
	}

	isCorrect := false
	if task.IsCorrect != nil {
		isCorrect = *task.IsCorrect
	}

	return c.JSON(http.StatusOK, contract.SubmitTaskResponse{
		Task:      taskResponse,
		IsCorrect: isCorrect,
	})
}
