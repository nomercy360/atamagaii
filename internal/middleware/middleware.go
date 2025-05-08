package middleware

import (
	"atamagaii/internal/contract"
	"context"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v4"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

func CustomHTTPErrorHandler(logger *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		statusCode := http.StatusInternalServerError
		var message interface{} = "Internal Server Error"
		var internalErr error

		var he *echo.HTTPError
		if errors.As(err, &he) {
			statusCode = he.Code
			if he.Message != nil {
				message = he.Message
			}
			internalErr = he.Internal
		} else {
			internalErr = err
		}

		logAttrs := []slog.Attr{
			slog.String("uri", c.Request().RequestURI),
			slog.Int("status", statusCode),
			slog.String("method", c.Request().Method),
		}

		if msgStr, ok := message.(string); ok {
			logAttrs = append(logAttrs, slog.String("message", msgStr))
			message = map[string]interface{}{"error": msgStr}
		}

		if internalErr != nil {
			logAttrs = append(logAttrs, slog.String("error", internalErr.Error()))
		}

		logger.LogAttrs(context.Background(), slog.LevelError, "REQUEST_ERROR", logAttrs...)

		var writeErr error
		if c.Request().Method == http.MethodHead {
			writeErr = c.NoContent(statusCode)
		} else {
			writeErr = c.JSON(statusCode, message)
		}

		if writeErr != nil {
			log.Printf("Error sending error response: %v", writeErr)
		}
	}
}

func GetUserAuthConfig(secret string) echojwt.Config {
	return echojwt.Config{
		NewClaimsFunc: func(_ echo.Context) jwt.Claims {
			return new(contract.JWTClaims)
		},
		SigningKey:             []byte(secret),
		ContinueOnIgnoredError: true,
		ErrorHandler: func(c echo.Context, err error) error {

			var extErr *echojwt.TokenExtractionError
			if !errors.As(err, &extErr) {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid auth token")
			}

			claims := &contract.JWTClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour * 30)),
				},
			}

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

			c.Set("user", token)

			if claims.UID == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid auth token")
			}

			return nil
		},
	}
}

func Setup(e *echo.Echo, logger *slog.Logger) {
	e.Use(echoMiddleware.Recover())
	e.Use(echoMiddleware.CORSWithConfig(echoMiddleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.PUT, echo.POST, echo.DELETE},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))
	e.Use(echoMiddleware.RequestLoggerWithConfig(echoMiddleware.RequestLoggerConfig{
		LogURI:       true,
		LogStatus:    true,
		LogMethod:    true,
		LogError:     true,
		LogRemoteIP:  true,
		LogUserAgent: true,
		HandleError:  true,
		LogValuesFunc: func(c echo.Context, v echoMiddleware.RequestLoggerValues) error {
			if v.Error == nil {
				logger.LogAttrs(c.Request().Context(), slog.LevelInfo, "REQUEST",
					slog.String("ip", v.RemoteIP),
					slog.String("method", v.Method),
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("user_agent", v.UserAgent),
				)
			}
			return nil
		},
	}))
	e.HTTPErrorHandler = CustomHTTPErrorHandler(logger)
	e.Use(echoMiddleware.TimeoutWithConfig(echoMiddleware.TimeoutConfig{
		Timeout: 20 * time.Second,
	}))
}
