package handler

import (
	"atamagaii/internal/contract"
	"atamagaii/internal/db"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	nanoid "github.com/matoous/go-nanoid/v2"
	initdata "github.com/telegram-mini-apps/init-data-golang"
	"log"
	"math/rand"
	"net/http"
	"time"
)

const (
	ErrInvalidInitData = "invalid init data"
	ErrInvalidRequest  = "invalid request"
)

func (h *Handler) TelegramAuth(c echo.Context) error {
	var req contract.AuthTelegramRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to bind request")
	}

	if err := req.Validate(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	log.Printf("AuthTelegram: %+v", req)

	expIn := 24 * time.Hour
	botToken := h.botToken

	if err := initdata.Validate(req.Query, botToken, expIn); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, ErrInvalidInitData)
	}

	data, err := initdata.Parse(req.Query)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, ErrInvalidInitData)
	}

	username := data.User.Username
	if username == "" {
		username = "user_" + fmt.Sprintf("%d", data.User.ID)
	}

	var name *string
	if data.User.FirstName != "" {
		name = &data.User.FirstName
		if data.User.LastName != "" {
			nameWithLast := fmt.Sprintf("%s %s", data.User.FirstName, data.User.LastName)
			name = &nameWithLast
		}
	}

	languageCode := "en"
	if data.User.LanguageCode != "" {
		languageCode = data.User.LanguageCode
	}

	user, err := h.db.GetUser(data.User.ID)
	if err != nil && errors.Is(err, db.ErrNotFound) {
		imgUrl := fmt.Sprintf("%s/avatars/%d.svg", "https://assets.peatch.io", rand.Intn(30)+1)
		create := db.User{
			ID:           nanoid.Must(),
			Username:     &username,
			TelegramID:   data.User.ID,
			Name:         name,
			AvatarURL:    &imgUrl,
			LanguageCode: languageCode,
			// Set default settings for new users
			Settings: &db.UserSettings{
				MaxTasksPerDay: 10, // Default to 10 tasks per day
				TaskTypes: []db.TaskType{
					db.TaskTypeVocabRecall,
					db.TaskTypeSentenceTranslation,
					db.TaskTypeAudio,
				},
			},
		}

		if err = h.db.SaveUser(&create); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to save user").SetInternal(err)
		}

		user, err = h.db.GetUser(data.User.ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user").SetInternal(err)
		}
	} else if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user").SetInternal(err)
	}

	if user.Username == nil {
		imgUrl := fmt.Sprintf("%s/avatars/%d.svg", "https://assets.peatch.io", rand.Intn(30)+1)

		upd := &db.User{
			TelegramID: data.User.ID,
			Username:   &username,
			Name:       name,
			AvatarURL:  &imgUrl,
		}

		if err = h.db.UpdateUser(upd); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to update user").SetInternal(err)
		}

		user, err = h.db.GetUser(data.User.ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user").SetInternal(err)
		}
	}

	token, err := generateJWT(user.ID, user.TelegramID, h.jwtSecret)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate JWT").SetInternal(err)
	}

	resp := contract.AuthTelegramResponse{
		Token: token,
		User:  *user,
	}

	return c.JSON(http.StatusOK, resp)
}

func generateJWT(userID string, chatID int64, secretKey string) (string, error) {
	claims := &contract.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
		UID:    userID,
		ChatID: chatID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	t, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}

	return t, nil
}