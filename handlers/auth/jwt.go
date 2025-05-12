package auth

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/gofiber/fiber/v2"
	"strings"
)

func RefreshAccessToken(c *fiber.Ctx) error {
	refreshHeader := c.Get("Authorization")

	if !strings.HasPrefix(refreshHeader, "Bearer ") {
		return c.Status(401).JSON(fiber.Map{"message": "Token non fourni ou invalide."})
	}

	refresh := strings.TrimPrefix(refreshHeader, "Bearer ")
	if refresh == "" {
		return c.Status(401).JSON(fiber.Map{"message": "Token vide."})
	}

	accessToken, newRefreshToken, err := utils.RefreshAccessToken(refresh)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"message": "Token invalide ou expiré."})
	}

	resp := fiber.Map{
		"access_token": accessToken,
	}
	if newRefreshToken != "" {
		resp["refresh_token"] = newRefreshToken
	}

	return c.Status(200).JSON(fiber.Map{
		"message": "✅ Nouveau token généré",
		"data":    resp,
	})
}
