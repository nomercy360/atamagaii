package contract

import "atamagaii/internal/db"

// UpdateUserSettingsRequest represents the request to update user settings
type UpdateUserSettingsRequest struct {
	MaxTasksPerDay int           `json:"max_tasks_per_day"`
	TaskTypes      []db.TaskType `json:"task_types"`
}

// UserSettingsResponse represents the user settings returned via API
type UserSettingsResponse struct {
	MaxTasksPerDay int      `json:"max_tasks_per_day"`
	TaskTypes      []string `json:"task_types"`
}

// UpdateUserSettingsRequest represents the request to update user settings within the update user API
type UpdateUserSettings struct {
	MaxTasksPerDay *int      `json:"max_tasks_per_day,omitempty"`
	TaskTypes      []string  `json:"task_types,omitempty"`
}

// UpdateUserRequest represents the request to update user profile information
type UpdateUserRequest struct {
	Name         *string            `json:"name,omitempty"`
	AvatarURL    *string            `json:"avatar_url,omitempty"`
	LanguageCode *string            `json:"language_code,omitempty"`
	Settings     *UpdateUserSettings `json:"settings,omitempty"`
}
