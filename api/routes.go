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
	ServersRoutes(servers)
	server := router.Group("/server/:serverId", middlewares.RequireAuth())
	ServerRoutes(server)
	debug := router.Group("/debug")
	DebugRoutes(debug)
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

func ServersRoutes(router fiber.Router) {
	// Création / lecture / mise à jour / suppression d’un serveur
	router.Post("/", handlers.CreateServer)  // POST   /servers
	router.Get("/", handlers.GetUserServers) // GET    /servers
}

func ServerRoutes(router fiber.Router) {
	router.Post("/join", handlers.JoinServer) // POST   /servers/:id/join
	router.Get("/", handlers.GetServer)       // GET    /servers/:id
	router.Patch("/", handlers.UpdateServer)  // PATCH  /servers/:id
	router.Delete("/", handlers.DeleteServer) // DELETE /servers/:id
	channels := router.Group("/channels", middlewares.RequireAuth())
	ChannelRoutes(channels)
}

func ChannelRoutes(router fiber.Router) {
	router.Get("/:id/messages", handlers.GetChannelMessages)
	router.Get("/", handlers.GetServerChannelsAndCategories)
	router.Post("/", handlers.CreateChannel)
	router.Patch("/:id", handlers.UpdateChannel)
	router.Delete("/:id", handlers.DeleteChannel)
}

// --- NOUVEAU : Fonction pour les routes de debug ---
func DebugRoutes(router fiber.Router) {
	router.Post("/seed/channels/:channelId", handlers.SeedChannelWithMessages)
}
