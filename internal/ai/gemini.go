package ai

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"atamagaii/internal/utils"
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/genai"
	"os"
)

const (
	// Default model for Gemini
	model = "gemini-2.5-pro-preview-06-05"
)

type GeminiClient struct {
	client    *genai.Client
	ttsClient *texttospeech.Client
	model     string
}

func NewGeminiClient(apiKey string) (*GeminiClient, error) {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	ttsClient, err := texttospeech.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google TTS client: %w", err)
	}

	return &GeminiClient{
		client:    client,
		ttsClient: ttsClient,
		model:     model,
	}, nil
}

func parseResponse[T any](text string) (T, error) {
	var result T
	err := json.Unmarshal([]byte(text), &result)
	if err != nil {
		return result, fmt.Errorf("error parsing response: %w", err)
	}
	return result, nil
}

func (c *GeminiClient) generateContent(
	ctx context.Context,
	prompt string,
	temperature float32,
	schema *genai.Schema,
) (string, error) {
	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](temperature),
		//MaxOutputTokens: 4096,
		TopP:           genai.Ptr[float32](0.95),
		ResponseSchema: schema,
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingBudget: genai.Ptr[int32](10000),
		},
		ResponseMIMEType: "application/json",
	}

	result, err := c.client.Models.GenerateContent(ctx, c.model, genai.Text(prompt), config)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	return result.Text(), nil
}

func (c *GeminiClient) GenerateCardContent(ctx context.Context, term string, language string) (*contract.CardFields, error) {
	responseSchema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"term": {
				Type: genai.TypeString,
			},
			"transcription": {
				Type: genai.TypeString,
			},
			"term_with_transcription": {
				Type: genai.TypeString,
			},
			"meaning_en": {
				Type: genai.TypeString,
			},
			"meaning_ru": {
				Type: genai.TypeString,
			},
			"example_native": {
				Type: genai.TypeString,
			},
			"example_en": {
				Type: genai.TypeString,
			},
			"example_ru": {
				Type: genai.TypeString,
			},
			"example_with_transcription": {
				Type: genai.TypeString,
			},
		},
		Required: []string{"term", "meaning_en", "meaning_ru", "example_native", "example_en", "example_ru", "example_with_transcription"},
	}

	prompt := fmt.Sprintf(`
Ты - языковой помощник, создающий карточки японских слов.

Используй словарную информацию, чтобы сгенерировать полноценную карточку с примерами, которые были бы понятны и полезны студенту японского.
Карточка должна быть в строгом формате JSON.

Требования к полям:
- Слово в поле term должно быть в словарной форме (для глаголов и прилагательных).
- Если у слова несколько значений, укажи наиболее употребимое или перечисли их кратко, если они просты и различны
- В term_with_transcription и example_with_transcription фуригану (транскрипцию) указывай только для иероглифов (漢字), используя формат 漢字[かな].
- Не добавляй транскрипцию к хирагане, катакане, частицам или целым словам, если это не кандзи.
- Например, правильно: とても寂[さび]しいです。Неправильно: とても寂しい[さびしい]です。
- Используй квадратные скобки [] и только формат 漢字[かな]. Не используй HTML, не используй круглые скобки.

Требования к примеру:
- Пример должен быть простым, понятными и близкими к повседневным ситуациям, чтобы ясно показывать значение и типичное употребление слова.
- Предложение должно быть коротким (10-12 слов).
---
Слово: %s
`, term)
	responseText, err := c.generateContent(ctx, prompt, 1.4, responseSchema)
	if err != nil {
		return nil, err
	}

	var vocabCard contract.CardFields
	vocabCard, err = parseResponse[contract.CardFields](responseText)
	if err != nil {
		return nil, fmt.Errorf("error parsing card content: %w", err)
	}

	// ensure no furigana in term field
	if vocabCard.Term != "" {
		vocabCard.Term = utils.RemoveFurigana(vocabCard.Term)
	}

	// ensure no furigana in examples
	if vocabCard.ExampleNative != "" {
		vocabCard.ExampleNative = utils.RemoveFurigana(vocabCard.ExampleNative)
	}

	return &vocabCard, nil
}

func (c *GeminiClient) GenerateTask(ctx context.Context, language, knownWords string, taskType db.TaskType) (*string, error) {
	prompt := fmt.Sprintf(`
Создай задание на перевод с русского на японский для учащегося, уровня N4-N5.
Условия:
• Используй НЕ БОЛЕЕ 1–2 слов из списка выученных слов, подходящие по контексту
Задание должно быть коротким (не более 7–9 слов).
Недавно изученные слова: %s
`, knownWords)

	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"sentence_ru": {
				Type: genai.TypeString,
			},
			"sentence_native": {
				Type: genai.TypeString,
			},
		},
	}

	if taskType == db.TaskTypeAudio {
		prompt = fmt.Sprintf(`
Create a JLPT N4-N5 Japanese listening comprehension task for a language learning app. The task should:

- Use a word [%s] that the user has recently learned.
- Create a short, natural 1-1.5 sentence monologue or story using the word.
- The story should be written in Japanese and suitable to be read aloud as audio.
- Ask a question in Japanese based on the story.
- Include correct answer in response.
- Vary the sentence structures, topics, and vocabulary to create diverse and engaging tasks.`, knownWords)
		schema = &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"story": {
					Type: genai.TypeString,
				},
				"question": {
					Type: genai.TypeString,
				},
				"correct_answer": {
					Type: genai.TypeString,
				},
			},
			Required: []string{"story", "question", "correct_answer"},
		}
	} else if taskType == db.TaskTypeVocabRecall {
		prompt = fmt.Sprintf(`
Create a Vocabulary Recall Test task for studying Japanese. 
The task should:
1. Be a text-based question asking the user to select the correct Japanese word (written in kanji or kana) for a given English meaning (e.g., “fast” → 速い).
2. Provide four multiple-choice answer options in Japanese, where, one option is the correct Japanese word.
3. The other three options are distractors that are designed to be tricky, such as:
   - Homophones or similar-sounding words with different meanings (e.g., 早い vs. 速い).
   - Words that are semantically related (e.g., 高い vs. 安い).
   - Common learner mistakes (e.g., using the wrong kanji for a known reading).
4. Use prefferably kanji, if its a not common or hard word, supply furigana in a square brackets e.g. 頭[あたま]がいい

The word to use is %s
`, knownWords)

		schema = &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"options": {
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"a": {
							Type: genai.TypeString,
						},
						"b": {
							Type: genai.TypeString,
						},
						"c": {
							Type: genai.TypeString,
						},
						"d": {
							Type: genai.TypeString,
						},
					},
					Required: []string{"a", "b", "c", "d"},
				},
				"correct_answer": {
					Type: genai.TypeString,
					Enum: []string{"a", "b", "c", "d"},
				},
			},
		}
	}

	responseText, err := c.generateContent(ctx, prompt, 1.4, schema)
	if err != nil {
		return nil, fmt.Errorf("error generating task content: %w", err)
	}

	return &responseText, nil
}

func (c *GeminiClient) GenerateAudio(ctx context.Context, text string, language string) (string, error) {
	voice := getGoogleTTSVoice(language)

	req := &texttospeechpb.SynthesizeSpeechRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Ssml{
				Ssml: fmt.Sprintf("<speak>%s</speak>", text),
			},
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: voice.LanguageCode,
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			EffectsProfileId: []string{"handset-class-device"},
			AudioEncoding:    texttospeechpb.AudioEncoding_LINEAR16,
			SpeakingRate:     1.0,
			Pitch:            0.0,
		},
	}

	response, err := c.ttsClient.SynthesizeSpeech(ctx, req)
	if err != nil {
		return "", fmt.Errorf("error generating audio with Google TTS: %w", err)
	}

	tempFile, err := os.CreateTemp("", "audio-*.wav")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %w", err)
	}
	defer tempFile.Close()

	if _, err := tempFile.Write(response.AudioContent); err != nil {
		return "", fmt.Errorf("error writing to temp file: %w", err)
	}

	return tempFile.Name(), nil
}

type TranslationCheckResult struct {
	Score    int     `json:"score"`
	Feedback *string `json:"feedback"`
}

func (c *GeminiClient) CheckSentenceTranslation(ctx context.Context, sentenceRu, correctAnswer, userAnswer string, languageCode string) (*TranslationCheckResult, error) {

	prompt := fmt.Sprintf(`
Ты - помощник по изучению японского языка. Проверь мой перевод с русского на японский.

Твоя задача:
1. Сравни мой ответ с правильным переводом.
2. Учитывай точность слов, порядок, частицы (は, が, を, に и т.д.), формы глаголов и стиль (вежливая форма и т.п.).
3. Оцени мой ответ по шкале от 0 до 100:
 • 100 — идеально, всё верно;
 • 90–99 — мелкие отличия, смысл сохранён;
 • 80–89 — есть ошибки, но фраза в целом понятна;
 • ниже 80 — серьёзные ошибки или искажение смысла.
4. Если оценка ниже 80, объясни в чём ошибка в 1–2 коротких предложениях.
---
Исходное предложение
%s
Мой перевод
%s
Эталонный перевод
%s`, sentenceRu, userAnswer, correctAnswer)
	responseSchema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"score": {
				Type: genai.TypeInteger,
			},
			"feedback": {
				Type: genai.TypeString,
			},
		},
		Required: []string{"score"},
	}

	responseText, err := c.generateContent(ctx, prompt, 0.3, responseSchema)
	if err != nil {
		return nil, fmt.Errorf("error generating translation check: %w", err)
	}

	var result TranslationCheckResult
	result, err = parseResponse[TranslationCheckResult](responseText)
	if err != nil {
		return nil, fmt.Errorf("error parsing translation check response: %w", err)
	}

	return &result, nil
}

type QuestionCheckResult struct {
	Score   int     `json:"score"`
	Comment *string `json:"comment"`
}

func (c *GeminiClient) CheckQuestionAnswer(ctx context.Context, question, userAnswer string, languageCode string) (*QuestionCheckResult, error) {

	prompt := fmt.Sprintf(`Вы — эксперт по японскому языку. Ученик ответил на вопрос на японском языке. Проверьте ответ на правильность, естественность и соответствие вопросу, добавьте оценку (0-100). Дайте только краткий комментарий на русском (1-2 предложения) об ошибках, если они есть, или о том, как сделать ответ более естественным для носителя языка. Не добавляйте общие суждения вроде «ответ верный» или «ответ понятен». Не усложняйте и не добавляйте лишней информации.

Вопрос: %s
Ответ ученика: %s`, question, userAnswer)
	responseSchema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"score": {
				Type: genai.TypeInteger,
			},
			"comment": {
				Type: genai.TypeString,
			},
		},
		Required: []string{"score"},
	}

	responseText, err := c.generateContent(ctx, prompt, 0.3, responseSchema)
	if err != nil {
		return nil, fmt.Errorf("error generating translation check: %w", err)
	}

	var result QuestionCheckResult
	result, err = parseResponse[QuestionCheckResult](responseText)
	if err != nil {
		return nil, fmt.Errorf("error parsing translation check response: %w", err)
	}

	return &result, nil
}

type StoryQuestionCheckResult struct {
	Score   int     `json:"score"`
	Comment *string `json:"comment"`
}

func (c *GeminiClient) CheckStoryQuestionAnswer(ctx context.Context, story, question, userAnswer string, languageCode string) (*StoryQuestionCheckResult, error) {
	prompt := fmt.Sprintf(`Вы — эксперт по японскому языку. Ученик ответил на вопрос на японском языке, основанный на истории. Проверьте ответ на правильность, естественность и соответствие вопросу, добавьте оценку (0-100). Дайте только краткий комментарий на русском (1-2 предложения) об ошибках, если они есть, или о том, как сделать ответ более естественным для носителя языка. Не добавляйте общие суждения вроде «ответ верный» или «ответ понятен». Не усложняйте и не добавляйте лишней информации.
История: %s
Вопрос: %s
Ответ ученика: %s`, story, question, userAnswer)
	responseSchema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"score": {
				Type: genai.TypeInteger,
			},
			"comment": {
				Type: genai.TypeString,
			},
		},
	}

	responseText, err := c.generateContent(ctx, prompt, 0.3, responseSchema)
	if err != nil {
		return nil, fmt.Errorf("error generating story question check: %w", err)
	}

	var result StoryQuestionCheckResult
	result, err = parseResponse[StoryQuestionCheckResult](responseText)
	if err != nil {
		return nil, fmt.Errorf("error parsing story question check response: %w", err)
	}

	return &result, nil
}

type GoogleTTSVoice struct {
	LanguageCode string
	Name         string
}

func getGoogleTTSVoice(language string) GoogleTTSVoice {
	switch language {
	case "jp":
		return GoogleTTSVoice{
			LanguageCode: "ja-JP",
			Name:         "ja-JP-Chirp3-HD-Puck",
		}
	case "ge":
		return GoogleTTSVoice{
			LanguageCode: "ka-GE",
			Name:         "ka-GE-Standard-A",
		}
	case "th":
		return GoogleTTSVoice{
			LanguageCode: "th-TH",
			Name:         "th-TH-Standard-A",
		}
	default:
		return GoogleTTSVoice{
			LanguageCode: "en-US",
			Name:         "en-US-Standard-A",
		}
	}
}

type CSVToJSONFields struct {
	Term struct {
		Value       string `json:"value"`
		ColumnIndex int    `json:"column_index"`
	} `json:"term"`
	Transcription struct {
		Value       string `json:"value"`
		ColumnIndex int    `json:"column_index"`
	} `json:"transcription"`
	TermWithTranscription struct {
		Value       string `json:"value"`
		ColumnIndex int    `json:"column_index"`
	} `json:"term_with_transcription"`
	MeaningEn struct {
		Value       string `json:"value"`
		ColumnIndex int    `json:"column_index"`
	} `json:"meaning_en"`
	MeaningRu struct {
		Value       string `json:"value"`
		ColumnIndex int    `json:"column_index"`
	} `json:"meaning_ru"`
	ExampleNative struct {
		Value       string `json:"value"`
		ColumnIndex int    `json:"column_index"`
	} `json:"example_native"`
	ExampleEn struct {
		Value       string `json:"value"`
		ColumnIndex int    `json:"column_index"`
	} `json:"example_en"`
	ExampleRu struct {
		Value       string `json:"value"`
		ColumnIndex int    `json:"column_index"`
	} `json:"example_ru"`
	ExampleWithTranscription struct {
		Value       string `json:"value"`
		ColumnIndex int    `json:"column_index"`
	} `json:"example_with_transcription"`
	Frequency struct {
		Value       int `json:"value"`
		ColumnIndex int `json:"column_index"`
	} `json:"frequency"`
	LanguageCode string `json:"language_code"`
}

func (c *GeminiClient) ParseCSVFields(ctx context.Context, text string) (CSVToJSONFields, error) {
	prompt := fmt.Sprintf(`
Сonvert to json
%s
`, text)
	responseSchema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"term": {
				Type:     genai.TypeObject,
				Required: []string{"value", "column_index"},
				Properties: map[string]*genai.Schema{
					"value": {
						Type: genai.TypeString,
					},
					"column_index": {
						Type: genai.TypeInteger,
					},
				},
			},
			"transcription": {
				Type:     genai.TypeObject,
				Required: []string{"value", "column_index"},
				Properties: map[string]*genai.Schema{
					"value": {
						Type: genai.TypeString,
					},
					"column_index": {
						Type: genai.TypeInteger,
					},
				},
			},
			"term_with_transcription": {
				Type:     genai.TypeObject,
				Required: []string{"value", "column_index"},
				Properties: map[string]*genai.Schema{
					"value": {
						Type: genai.TypeString,
					},
					"column_index": {
						Type: genai.TypeInteger,
					},
				},
			},
			"meaning_en": {
				Type:     genai.TypeObject,
				Required: []string{"value", "column_index"},
				Properties: map[string]*genai.Schema{
					"value": {
						Type: genai.TypeString,
					},
					"column_index": {
						Type: genai.TypeInteger,
					},
				},
			},
			"meaning_ru": {
				Type:     genai.TypeObject,
				Required: []string{"value", "column_index"},
				Properties: map[string]*genai.Schema{
					"value": {
						Type: genai.TypeString,
					},
					"column_index": {
						Type: genai.TypeInteger,
					},
				},
			},
			"example_native": {
				Type:     genai.TypeObject,
				Required: []string{"value", "column_index"},
				Properties: map[string]*genai.Schema{
					"value": {
						Type: genai.TypeString,
					},
					"column_index": {
						Type: genai.TypeInteger,
					},
				},
			},
			"example_en": {
				Type:     genai.TypeObject,
				Required: []string{"value", "column_index"},
				Properties: map[string]*genai.Schema{
					"value": {
						Type: genai.TypeString,
					},
					"column_index": {
						Type: genai.TypeInteger,
					},
				},
			},
			"example_ru": {
				Type:     genai.TypeObject,
				Required: []string{"value", "column_index"},
				Properties: map[string]*genai.Schema{
					"value": {
						Type: genai.TypeString,
					},
					"column_index": {
						Type: genai.TypeInteger,
					},
				},
			},
			"example_with_transcription": {
				Type:     genai.TypeObject,
				Required: []string{"value", "column_index"},
				Properties: map[string]*genai.Schema{
					"value": {
						Type: genai.TypeString,
					},
					"column_index": {
						Type: genai.TypeInteger,
					},
				},
			},
			"frequency": {
				Type:     genai.TypeObject,
				Required: []string{"value", "column_index"},
				Properties: map[string]*genai.Schema{
					"value": {
						Type: genai.TypeInteger,
					},
					"column_index": {
						Type: genai.TypeInteger,
					},
				},
			},
			"language_code": {
				Type: genai.TypeString,
			},
		},
		Required: []string{
			"term", "language_code",
		},
	}

	responseText, err := c.generateContent(ctx, prompt, 0, responseSchema)
	if err != nil {
		return CSVToJSONFields{}, fmt.Errorf("error generating gemini mapping: %w", err)
	}
	//
	var fields CSVToJSONFields
	fields, err = parseResponse[CSVToJSONFields](responseText)
	if err != nil {
		return CSVToJSONFields{}, fmt.Errorf("error parsing gemini mapping response: %w", err)
	}

	return fields, nil
}
