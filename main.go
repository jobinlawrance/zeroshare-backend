package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"zeroshare-backend/config"
	controller "zeroshare-backend/controllers"
	"zeroshare-backend/middlewares"
	"zeroshare-backend/structs"
)

var DB *gorm.DB
var redisStore *redis.Client

func shoudSkipPath(c *fiber.Ctx) bool {
	// Skip authentication for login and callback routes
	path := c.Path()

	// Handle dynamic routes manually (e.g., /login/qr/:token)
	if path == "/oauth/google" || path == "/auth/google/callback" ||
		strings.HasPrefix(path, "/login/") ||
		strings.HasPrefix(path, "/sse/") {
		return true
	}
	return false
}

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

	// JWT Middleware
	app.Use(func(c *fiber.Ctx) error {
		if shoudSkipPath(c) {
			// Skip JWT authentication for these paths
			return c.Next()
		}
		// Otherwise, apply JWT middleware
		return middlewares.NewAuthMiddleware(config.Secret)(c)
	})

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

	app.Post("/node", func(c *fiber.Ctx) error {
		response := new(structs.NodeResponse)
		json.Unmarshal(c.Body(), response)
		userId, _ := controller.GetFromToken(c, "ID")
		uid, _ := uuid.Parse(userId.(string))
		peer := structs.Peer{
			MachineName: response.MachineName,
			NetworkId:   response.NetworkId,
			NodeId:      response.NodeId,
			UserId:      uid,
		}

		controller.AddPeerAndAuthorize(context.Background(), peer, DB)

		return c.SendStatus(fiber.StatusOK)
	})

	app.Listen(":4000")
}
