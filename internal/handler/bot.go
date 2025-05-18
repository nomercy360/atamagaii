package handler

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"atamagaii/internal/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/labstack/echo/v4"
	nanoid "github.com/matoous/go-nanoid/v2"
	"log"
	"math/rand"
	"strings"
)

func (h *Handler) HandleWebhook(c echo.Context) error {
	var update tgbotapi.Update
	if err := c.Bind(&update); err != nil {
		log.Printf("Failed to bind update: %v", err)
		return c.NoContent(400)
	}

	if update.Message == nil && update.CallbackQuery == nil {
		return c.NoContent(200)
	}

	resp := h.handleUpdate(update)
	if _, err := h.bot.SendMessage(context.Background(), resp); err != nil {
		log.Printf("Failed to send message: %v", err)
	}

	return c.NoContent(200)
}

func (h *Handler) handleUpdate(update tgbotapi.Update) (msg *telegram.SendMessageParams) {
	var chatID int64
	var name *string
	var username *string
	if update.Message != nil {
		chatID = update.Message.From.ID
		username = &update.Message.From.UserName

		name = &update.Message.From.FirstName
		if update.Message.From.FirstName != "" {
			name = &update.Message.From.FirstName
			if update.Message.From.LastName != "" {
				nameWithLast := fmt.Sprintf("%s %s", update.Message.From.FirstName, update.Message.From.LastName)
				name = &nameWithLast
			}
		}
	}

	if username == nil {
		usernameFromID := fmt.Sprintf("user_%d", chatID)
		username = &usernameFromID
	}

	user, err := h.db.GetUser(chatID)

	msg = &telegram.SendMessageParams{
		ChatID: chatID,
	}

	if err != nil && errors.Is(err, db.ErrNotFound) {
		languageCode := "en"
		if update.Message != nil && update.Message.From.LanguageCode != "" {
			languageCode = update.Message.From.LanguageCode
		}

		imgUrl := fmt.Sprintf("%s/avatars/%d.svg", "https://assets.peatch.io", rand.Intn(30)+1)

		newUser := &db.User{
			ID:           nanoid.Must(),
			TelegramID:   chatID,
			Username:     username,
			Name:         name,
			AvatarURL:    &imgUrl,
			LanguageCode: languageCode,
		}

		if err := h.db.SaveUser(newUser); err != nil {
			log.Printf("Failed to save user: %v", err)
			msg.Text = "Ошибка при регистрации пользователя. Попробуй позже."
		} else {
			msg.Text = "Добро пожаловать! Используй /start для начала работы с ботом."
		}

		user, err = h.db.GetUser(chatID)
		if err != nil {
			log.Printf("Failed to get user after saving: %v", err)
			msg.Text = "Ошибка при получении пользователя. Попробуй позже."
		}
	} else if err != nil {
		log.Printf("Failed to get user: %v", err)
		msg.Text = "Ошибка при получении пользователя. Попробуй позже."
	} else if user.AvatarURL == nil {
		imgUrl := fmt.Sprintf("%s/avatars/%d.svg", "https://assets.peatch.io", rand.Intn(30)+1)

		newUser := &db.User{
			TelegramID: chatID,
			Username:   username,
			Name:       name,
			AvatarURL:  &imgUrl,
		}

		if err := h.db.UpdateUser(newUser); err != nil {
			log.Printf("Failed to update user: %v", err)
		}
	}

	if update.Message == nil || user == nil {
		return msg
	}

	if update.Message.IsCommand() {
		switch update.Message.Command() {
		case "start":
			msg.Text = "Привет\\! Этот бот для изучения японского языка\\. Он поможет тебе практиковать слов и грамматику\\!\n\n"
			msg.ParseMode = models.ParseModeMarkdown
		case "help":
			msg.Text = "Просто отправь мне слово или фразу на любом языке, и я создам для тебя карточку для изучения\\!"
			msg.ParseMode = models.ParseModeMarkdown
		default:
			msg.Text = "Неизвестная команда. Используй /help для получения справки."
		}
		return msg
	}

	if update.Message.Text != "" {
		msgText := strings.TrimSpace(update.Message.Text)

		if len(msgText) < 2 {
			msg.Text = "Сообщение слишком короткое для создания карточки. Пожалуйста, отправь более длинный текст."
			return msg
		}

		cardResp, lang, err := h.createCardFromMessage(user.ID, msgText)
		if err != nil {
			log.Printf("Failed to create card from message: %v", err)
			msg.Text = "Не удалось создать карточку. Попробуй позже."
			return msg
		}

		languageName := utils.GetLanguageNameFromCode(lang)
		msg.Text = fmt.Sprintf("Создана новая карточка для изучения \\(%s\\):\n\n*%s*",
			languageName,
			telegram.EscapeMarkdown(cardResp.Fields.Term))
		msg.ParseMode = models.ParseModeMarkdown

		button := models.InlineKeyboardButton{
			Text:   "Посмотреть карточку",
			WebApp: &models.WebAppInfo{URL: fmt.Sprintf("%s/edit-card/%s/%s", h.webAppURL, cardResp.DeckID, cardResp.ID)},
		}

		msg.ReplyMarkup = models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{button},
			},
		}

		return msg
	}

	if msg.Text == "" {
		msg.Text = "Отправь мне слово или фразу, чтобы создать карточку для изучения!"
	}

	return msg
}

func (h *Handler) createCardFromMessage(userID string, messageText string) (*contract.CardResponse, string, error) {
	languageCode := utils.DetectLanguage(messageText)

	transcriptionType := utils.GetDefaultTranscriptionType(languageCode)

	deck, err := h.db.GetOrCreateGeneratedDeck(userID, languageCode, transcriptionType)
	if err != nil {
		return nil, "", fmt.Errorf("error getting/creating deck: %w", err)
	}

	cardFields := contract.CardFields{
		Term: messageText,
	}

	fieldsJSON, err := json.Marshal(cardFields)
	if err != nil {
		return nil, "", fmt.Errorf("error marshalling card fields: %w", err)
	}

	card, err := h.db.AddCard(userID, deck.ID, string(fieldsJSON))
	if err != nil {
		return nil, "", fmt.Errorf("error adding card: %w", err)
	}

	cardResponse, err := formatCardResponse(*card)
	if err != nil {
		return nil, "", fmt.Errorf("error formatting card response: %w", err)
	}

	return &cardResponse, languageCode, nil
}
