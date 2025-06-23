package handlers

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils/dbTools"
	"github.com/gofiber/fiber/v2"
	"strings"
)

func Me(c *fiber.Ctx) error {
	auth := c.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return c.Status(401).JSON(fiber.Map{"message": "Vous n'êtes pas authentifié !"})
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" {
		return c.Status(401).JSON(fiber.Map{"message": "Vous n'êtes pas authentifié !"})
	}
	userId, isRefresh := utils.CheckUserToken(token)
	if isRefresh {
		return c.Status(401).JSON(fiber.Map{"token_expired": true, "message": "Token d'accès expiré, veuillez vous reconnecter."})
	}
	if userId == nil {
		return c.Status(401).JSON(fiber.Map{"message": "Vous n'êtes pas authentifié !"})
	}
	data, err := dbTools.GetUserByID(userId)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"message": "Impossible de récupérer vos données utilisateur !"})
	}
	userData := fiber.Map{
		"id":       userId,
		"email":    data.Email,
		"username": data.Username,
		"avatar":   data.Avatar,
	}

	return c.Status(200).JSON(fiber.Map{
		"message": "Utilisateur authentifié !",
		"status":  "success",
		"data":    userData,
	})

}
