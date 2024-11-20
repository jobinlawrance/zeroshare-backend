package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

var DB *gorm.DB

func main() {
	app := fiber.New()
	app.Use(logger.New())
	app.Static("/assets", "./assets")

	er := godotenv.Load(".env")
	if er != nil {
		log.Fatal("Dayum, Error loading .env file:", er)
	}

	

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	app.Listen(":3000")
}
