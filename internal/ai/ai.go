package ai

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"context"
)

type AIClient interface {
	GenerateCardContent(ctx context.Context, term string, language string) (*contract.CardFields, error)
	GenerateTask(ctx context.Context, language, templateName string, taskType db.TaskType) (*string, error)
	GenerateAudio(ctx context.Context, text string, language string) (string, error)
	CheckSentenceTranslation(ctx context.Context, sentenceRu, correctAnswer, userAnswer string, languageCode string) (*TranslationCheckResult, error)
	ParseCSVFields(ctx context.Context, line string) (CSVToJSONFields, error)
	CheckQuestionAnswer(ctx context.Context, question, answer, languageCode string) (*QuestionCheckResult, error)
	CheckStoryQuestionAnswer(ctx context.Context, story, question, userAnswer string, languageCode string) (*StoryQuestionCheckResult, error)
}
