package api

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/handlers"
	"github.com/Romain-GUILLEMOT/WhispyrBack/handlers/auth"
	middlewares "github.com/Romain-GUILLEMOT/WhispyrBack/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	router := app.Group("/api")
	router.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("✅ API en bonne santé !")
	})
	router.Get("/me", middlewares.RequireAuth(), handlers.Me)

	auth := router.Group("/auth")
	AuthRoutes(auth)
}

func AuthRoutes(router fiber.Router) {
	router.Get("/code", auth.RegisterAskCode)
	router.Post("/code", auth.RegisterVerifyCode)
	router.Post("/register", auth.RegisterUser)
	router.Get("/refresh", auth.RefreshAccessToken)

}
