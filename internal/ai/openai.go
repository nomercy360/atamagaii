package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type VocabularyCard struct {
	Term                     string `json:"term" jsonschema_description:"The primary term in native script"`
	Transcription            string `json:"transcription" jsonschema_description:"Reading aid (pinyin, romaji, etc.)"`
	TermWithTranscription    string `json:"term_with_transcription" jsonschema_description:"Term with reading aids embedded (e.g., in furigana format with square brackets for Japanese)"`
	MeaningEn                string `json:"meaning_en" jsonschema_description:"English translation of the term"`
	MeaningRu                string `json:"meaning_ru" jsonschema_description:"Russian translation of the term"`
	ExampleNative            string `json:"example_native" jsonschema_description:"Example sentence in native script"`
	ExampleEn                string `json:"example_en" jsonschema_description:"English translation of the example sentence"`
	ExampleRu                string `json:"example_ru" jsonschema_description:"Russian translation of the example sentence"`
	ExampleWithTranscription string `json:"example_with_transcription" jsonschema_description:"Example with reading aids (furigana or transliteration)"`
	Frequency                int    `json:"frequency" jsonschema_description:"Usage frequency data, 1-3000 (1=very common)"`
}

func GenerateSchema[T any]() interface{} {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}

var vocabularyCardSchema = GenerateSchema[VocabularyCard]()

type OpenAIClient struct {
	client openai.Client
}

func NewOpenAIClient(apiKey string) (*OpenAIClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &OpenAIClient{
		client: client,
	}, nil
}

func (c *OpenAIClient) GenerateCardContent(ctx context.Context, term string, language string) (map[string]interface{}, error) {
	examples, systemPrompt := getFewShotExamples(language)
	messages := []openai.ChatCompletionMessageParamUnion{
		{
			OfSystem: &openai.ChatCompletionSystemMessageParam{
				Content: openai.ChatCompletionSystemMessageParamContentUnion{
					OfString: openai.String(systemPrompt),
				},
			},
		},
	}

	for _, example := range examples {
		messages = append(messages, example...)
	}

	messages = append(messages, openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfString: openai.String(fmt.Sprintf(`{"term": "%s"}`, term)),
			},
		},
	})

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "vocabulary_card",
		Description: openai.String("Structure for a language vocabulary flashcard"),
		Schema:      vocabularyCardSchema,
		Strict:      openai.Bool(true),
	}

	response, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:       openai.ChatModelGPT4oMini,
		Messages:    messages,
		Temperature: openai.Float(0.7),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: schemaParam,
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("error generating card content: %w", err)
	}

	content := response.Choices[0].Message.Content

	var vocabularyCard VocabularyCard
	if err := json.Unmarshal([]byte(content), &vocabularyCard); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return map[string]interface{}{
		"raw_response": content,
		"card":         vocabularyCard,
	}, nil
}

func (c *OpenAIClient) GenerateAudio(ctx context.Context, text string, language string) (string, error) {
	params := openai.AudioSpeechNewParams{
		Model:          openai.SpeechModelGPT4oMiniTTS,
		Input:          text,
		Voice:          openai.AudioSpeechNewParamsVoiceOnyx,
		ResponseFormat: openai.AudioSpeechNewParamsResponseFormatAAC,
		Speed:          openai.Float(1.0),
		Instructions:   openai.String(getAudioInstructions(language)),
	}

	response, err := c.client.Audio.Speech.New(ctx, params)

	if err != nil {
		return "", fmt.Errorf("error generating audio: %w", err)
	}

	tempFile, err := os.CreateTemp("", "audio-*.mp3")
	if err != nil {
		return "", fmt.Errorf("error creating temp file: %w", err)
	}
	defer tempFile.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}
	defer response.Body.Close()

	if _, err := tempFile.Write(body); err != nil {
		return "", fmt.Errorf("error writing to temp file: %w", err)
	}

	return tempFile.Name(), nil
}

func getFewShotExamples(language string) ([][]openai.ChatCompletionMessageParamUnion, string) {

	var examples [][]openai.ChatCompletionMessageParamUnion
	var systemPrompt string

	switch language {
	case "japanese":
		systemPrompt = "You're a language assistant creating Japanese flashcards for JLPT N5 level. Use the provided term to generate a complete flashcard with examples that would be clear and useful for a beginner Japanese student. The card must be in strict JSON format as shown in the examples. Requirements for examples: Keep sentences short (10-12 words max). Use only JLPT N5 level grammar and vocabulary. Use natural context and the word in its most typical form. For furigana in term_with_transcription and example_with_transcription, use square brackets format: 漢字[かな]. Don't use HTML or parentheses - only []."

		examples = append(examples, []openai.ChatCompletionMessageParamUnion{
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(`{"term": "食べる"}`),
					},
				},
			},
			{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(`{
						"term": "食べる",
						"transcription": "たべる",
						"term_with_transcription": "食[たべ]る",
						"meaning_en": "to eat",
						"meaning_ru": "есть, кушать",
						"example_native": "毎朝、パンを食べます。",
						"example_en": "I eat bread every morning.",
						"example_ru": "Я ем хлеб каждое утро.",
						"example_with_transcription": "毎朝[まいあさ]、パンを食[たべ]ます。",
						"frequency": 542
					}`),
					},
				},
			},
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(`{"term": "学生"}`),
					},
				},
			},
			{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(`{
						"term": "学生",
						"transcription": "がくせい",
						"term_with_transcription": "学生[がくせい]",
						"meaning_en": "student",
						"meaning_ru": "студент",
						"example_native": "彼は日本の学生です。",
						"example_en": "He is a student in Japan.",
						"example_ru": "Он студент в Японии.",
						"example_with_transcription": "彼[かれ]は日本[にほん]の学生[がくせい]です。",
						"frequency": 321
					}`),
					},
				},
			},
		})
	case "georgian":
		systemPrompt = "You're a language assistant creating Georgian flashcards for beginners. Use the provided term to generate a complete flashcard with examples that would be clear and useful for a beginner Georgian language student. The card must be in strict JSON format as shown in the examples. Requirements for examples: Keep sentences short (10-12 words max). Use only beginner level grammar and vocabulary. Use natural context and the word in its most typical form. For example_with_transcription, use Latin letters to convey Georgian pronunciation. Don't use HTML or special formats - just plain Latin text."

		examples = append(examples, []openai.ChatCompletionMessageParamUnion{
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(`{"term": "სახლი"}`),
					},
				},
			},
			{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(`{
						"term": "სახლი",
						"transcription": "sakhli",
						"meaning_en": "house, home",
						"meaning_ru": "дом, жилище",
						"example_native": "ეს ჩემი სახლია.",
						"example_en": "This is my house.",
						"example_ru": "Это мой дом.",
						"example_with_transcription": "es chemi sakhlia.",
						"frequency": 215
					}`),
					},
				},
			},
		})
	case "thai":
		systemPrompt = "You're a language assistant creating Thai flashcards for beginners. Use the provided term to generate a complete flashcard with examples that would be clear and useful for a beginner Thai language student. The card must be in strict JSON format as shown in the examples. Requirements for examples: Keep sentences short (10-12 words max). Use only beginner level grammar and vocabulary. Use natural context and the word in its most typical form. For transcription, use the American University Alumni / Peace Corps system to reflect pronunciation. Don't use HTML or special formats - just plain text."

		examples = append(examples, []openai.ChatCompletionMessageParamUnion{
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(`{"term": "แมว"}`),
					},
				},
			},
			{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(`{
						"term": "แมว",
						"transcription": "mɛɛw",
						"meaning_en": "cat",
						"meaning_ru": "кот, кошка",
						"example_native": "แมวของฉันชอบนอนบนเตียง",
						"example_en": "My cat likes to sleep on the bed.",
						"example_ru": "Моя кошка любит спать на кровати.",
						"example_with_transcription": "mɛɛw khɔ̌ɔŋ chǎn chɔ̂ɔp nɔɔn bon tiaŋ",
						"frequency": 150
					}`),
					},
				},
			},
			{
				OfUser: &openai.ChatCompletionUserMessageParam{
					Content: openai.ChatCompletionUserMessageParamContentUnion{
						OfString: openai.String(`{"term": "เรียน"}`),
					},
				},
			},
			{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: openai.String(`{
						"term": "เรียน",
						"transcription": "rian",
						"meaning_en": "to study, to learn",
						"meaning_ru": "учиться, изучать",
						"example_native": "เด็กๆเรียนภาษาอังกฤษที่โรงเรียน",
						"example_en": "Children study English at school.",
						"example_ru": "Дети учат английский в школе.",
						"example_with_transcription": "dèk dèk rian phaasǎa aŋkrìt thîi rooŋ rian",
						"frequency": 210
					}`),
					},
				},
			},
		})
	default:
		return getFewShotExamples("japanese")
	}

	return examples, systemPrompt
}

func getAudioInstructions(language string) string {
	switch language {
	case "japanese":
		return "Speak in natural Japanese with clear pronunciation."
	case "georgian":
		return "Speak in natural Georgian with clear pronunciation."
	case "thai":
		return "Speak in natural Thai with clear pronunciation."
	default:
		return "Speak with clear pronunciation."
	}
}
