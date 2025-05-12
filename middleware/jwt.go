package middlewares

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func RequireAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		auth := c.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Token d'accès manquant ou mal formé.",
			})
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		userID, isRefresh := utils.CheckUserToken(token)

		if userID == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Token invalide ou utilisateur introuvable.",
			})
		}

		if isRefresh {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"token_expired": true,
				"message":       "Token expiré. Merci de le rafraîchir.",
			})
		}

		c.Locals("user_id", userID)

		return c.Next()
	}
}
