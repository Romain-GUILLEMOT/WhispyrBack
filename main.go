package main

import (
	"github.com/Romain-GUILLEMOT/WhispyrBack/api"
	"github.com/Romain-GUILLEMOT/WhispyrBack/config"
	"github.com/Romain-GUILLEMOT/WhispyrBack/db"
	"github.com/Romain-GUILLEMOT/WhispyrBack/handlers"
	"github.com/Romain-GUILLEMOT/WhispyrBack/utils"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	defer utils.HandlePanic()
	if err := godotenv.Load(); err != nil {
		log.Fatal(".env introuvable.")
	}
	app := fiber.New(fiber.Config{
		BodyLimit: 10 * 1024 * 1024, // 10 MB
	})
	debug := os.Getenv("APP_DEBUG")
	if debug == "true" {
		log.Println("Running in debug mode")
		app.Use(logger.New())

	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://whispyr.romain-guillemot.dev",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Requested-With", // Add X-Requested-With here		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowCredentials: true,
	}))
	utils.InitLogger()

	config.LoadConfig()
	db.ConnectDB()
	db.ApplyMigrations(db.Session)
	utils.MinioInit()
	utils.InitRedis()
	utils.InitMailer()
	handlers.StartBroadcaster()

	api.SetupRoutes(app)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Fatal(app.Listen(":" + port))

}
