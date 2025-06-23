package middlewares

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// WebSocketAuth middleware : vérifie le token passé en query ?token=<JWT>
func WebSocketAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Query("token")

		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Token d'accès WebSocket manquant.",
			})
		}

		userID, isRefresh := utils.CheckUserToken(token)
		if userID == nil || isRefresh {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Token invalide ou expiré.",
			})
		}

		c.Locals("user_id", userID)

		// Upgrade WebSocket si tout est OK
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}

		return fiber.ErrUpgradeRequired
	}
}
