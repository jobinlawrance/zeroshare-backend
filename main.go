package main

import (
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"gorm.io/gorm"

	controller "zeroshare-backend/controllers"
)

var DB *gorm.DB
var redisStore *redis.Client

func main() {
	app := fiber.New()
	app.Use(logger.New())
	app.Static("/assets", "./assets")

	er := godotenv.Load(".env")
	if er != nil {
		log.Fatal("Dayum, Error loading .env file:", er)
	}

	DB = controller.InitDatabase()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*", // Adjust this to allow only specific origins if needed
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Authorization, Content-Type, Accept",
	}))

	oauthConf := controller.SetUpOAuth()

	redisStore = controller.SetupRedis()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	app.Get("/sse/:sessionToken", func(c *fiber.Ctx) error {
		sessionToken := c.Params("sessionToken")
		return controller.SSE(c, redisStore, sessionToken)
	})

	app.Get("/login/:token", func(c *fiber.Ctx) error {
		sessionToken := c.Params("token")
		url := oauthConf.AuthCodeURL(sessionToken)
		return c.Redirect(url)
	})

	app.Get("auth/google/callback", func(c *fiber.Ctx) error {
		//get code from query params for generating token
		return controller.GetAuthData(c, oauthConf, DB, redisStore)
	})

	app.Listen(":4000")
}
