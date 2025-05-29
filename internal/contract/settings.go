package contract

import "atamagaii/internal/db"

type UpdateUserSettingsRequest struct {
	MaxTasksPerDay int           `json:"max_tasks_per_day"`
	TaskTypes      []db.TaskType `json:"task_types"`
}

type UserSettingsResponse struct {
	MaxTasksPerDay int      `json:"max_tasks_per_day"`
	TaskTypes      []string `json:"task_types"`
}

type UpdateUserSettings struct {
	MaxTasksPerDay *int     `json:"max_tasks_per_day,omitempty"`
	TaskTypes      []string `json:"task_types,omitempty"`
}

type UpdateUserRequest struct {
	Name         *string             `json:"name,omitempty"`
	AvatarURL    *string             `json:"avatar_url,omitempty"`
	LanguageCode *string             `json:"language_code,omitempty"`
	Settings     *UpdateUserSettings `json:"settings,omitempty"`
}
