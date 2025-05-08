package handlers

import (
	"atamagaii/internal/db"
	"context"
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	telegram "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/labstack/echo/v4"
	nanoid "github.com/matoous/go-nanoid/v2"
	"log"
	"math/rand"
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
		imgUrl := fmt.Sprintf("%s/avatars/%d.svg", "https://assets.peatch.io", rand.Intn(30)+1)

		newUser := &db.User{
			ID:         nanoid.Must(),
			TelegramID: chatID,
			Username:   username,
			Name:       name,
			AvatarURL:  &imgUrl,
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

	switch update.Message.Command() {
	case "start":
		msg.Text = "Привет\\! Этот бот для изучения японского языка\\. Он поможет тебе практиковать слов и грамматику\\!\n\n"
		msg.ParseMode = models.ParseModeMarkdown
	default:

	}

	return msg
}
