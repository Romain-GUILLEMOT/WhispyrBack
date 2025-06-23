package api

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/handlers"
	"github.com/Romain-GUILLEMOT/WhispyrBack/handlers/auth"
	middlewares "github.com/Romain-GUILLEMOT/WhispyrBack/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

func SetupRoutes(app *fiber.App) {
	router := app.Group("/api")
	router.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("✅ API en bonne santé !")
	})
	router.Get("/me", middlewares.RequireAuth(), handlers.Me)

	auth := router.Group("/auth")
	AuthRoutes(auth)
	servers := router.Group("/servers", middlewares.RequireAuth())
	ServerRoutes(servers)
	router.Use("/ws", middlewares.WebSocketAuth(), handlers.WebSocketHandler)
	router.Get("/ws", middlewares.WebSocketAuth(), websocket.New(handlers.HandleWebSocket))

}

func AuthRoutes(router fiber.Router) {
	router.Get("/code", auth.RegisterAskCode)
	router.Post("/code", auth.RegisterVerifyCode)
	router.Post("/register", auth.RegisterUser)
	router.Post("/login", auth.LoginUser)
	router.Get("/refresh", auth.RefreshAccessToken)

}

func ServerRoutes(router fiber.Router) {
	// Création / lecture / mise à jour / suppression d’un serveur
	router.Post("/", handlers.CreateServer)      // POST   /servers
	router.Get("/", handlers.GetUserServers)     // GET    /servers     ← liste *tous* les serveurs liés à l’utilisateur
	router.Get("/:id", handlers.GetServer)       // GET    /servers/:id
	router.Patch("/:id", handlers.UpdateServer)  // PATCH  /servers/:id
	router.Delete("/:id", handlers.DeleteServer) // DELETE /servers/:id

	// Joindre un serveur existant
	router.Post("/:id/join", handlers.JoinServer) // POST   /servers/:id/join

	// NOTE: on a supprimé /owned et /member, tout est maintenant via GET /
}
