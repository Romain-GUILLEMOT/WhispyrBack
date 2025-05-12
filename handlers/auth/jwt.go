package auth

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/gofiber/fiber/v2"
	"strings"
)

func RefreshAccessToken(c *fiber.Ctx) error {
	accessHeader := c.Get("Authorization")
	refreshHeader := c.Get("X-Refresh-Token")

	if !strings.HasPrefix(accessHeader, "Bearer ") || !strings.HasPrefix(refreshHeader, "Bearer ") {
		return c.Status(401).JSON(fiber.Map{"message": "Vous n'êtes pas authentifié !"})
	}
	access := strings.TrimPrefix(accessHeader, "Bearer ")
	refresh := strings.TrimPrefix(refreshHeader, "Bearer ")

	if access == "" {
		return c.Status(401).JSON(fiber.Map{"message": "Vous n'êtes pas authentifié !"})
	}
	isRefresh, err := utils.ShouldRefreshAccessToken(access)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"message": "Vous n'êtes pas authentifié !"})
	}
	if !isRefresh {
		return c.Status(401).JSON(fiber.Map{"message": "Votre token est déja valide !"})
	}
	accessToken, refreshToken, err := utils.RefreshAccessToken(refresh)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"message": "Vous n'êtes pas authentifié !"})
	}
	if accessToken == "" {
		return c.Status(401).JSON(fiber.Map{"message": "Vous n'êtes pas authentifié !"})
	}
	return c.Status(200).JSON(fiber.Map{"message": "Votre token a été rafraichi !", "data": fiber.Map{"access_token": accessToken, "refresh_token": refreshToken}})

}
