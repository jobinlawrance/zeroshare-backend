package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	controller "zeroshare-backend/controllers"
	"zeroshare-backend/middlewares"
	"zeroshare-backend/structs"

	pb "zeroshare-backend/proto"
)

var DB *gorm.DB
var redisStore *redis.Client

func shoudSkipPath(c *fiber.Ctx) bool {
	// Skip authentication for login and callback routes
	path := c.Path()

	// Handle dynamic routes manually (e.g., /login/qr/:token)
	if path == "/oauth/google" || path == "/auth/google/callback" ||
		strings.HasPrefix(path, "/login/") ||
		strings.HasPrefix(path, "/proxy/") ||
		strings.HasPrefix(path, "/refresh") ||
		strings.HasPrefix(path, "/stream") ||
		strings.HasPrefix(path, "/notification-tone") ||
		strings.HasPrefix(path, "/sse/") {
		return true
	}
	return false
}

func main() {
	app := fiber.New()
	app.Use(logger.New())
	app.Static("/assets", "./assets")

	// Load environment variables from .env file if not running in a containerized environment
	if os.Getenv("APP_ENV") != "production" {
		err := godotenv.Load(".env")
		if err != nil {
			log.Fatal("Dayum, Error loading .env file:", err)
		}
	}

	DB = controller.InitDatabase()
	go pb.StartGRPCServer(DB)

	controller.InitNebula(context.Background())

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
		return middlewares.NewAuthMiddleware(os.Getenv("AUTH_SECRET"))(c)
	})

	app.Use("/stream", func(c *fiber.Ctx) error {
		log.Println("Incoming request:", c.Method(), c.Path())
		// IsWebSocketUpgrade returns true if the client
		// requested upgrade to the WebSocket protocol.
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
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
			Platform:    response.Platform,
		}

		controller.AddPeerAndAuthorize(context.Background(), peer, DB)

		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/device", func(c *fiber.Ctx) error {
		log.Println("Request received: ", c.Method())
		log.Println("Request body: ", string(c.Body())) // Log the raw body
		response := new(structs.Device)

		// Unmarshal the body and handle the error
		if err := json.Unmarshal(c.Body(), response); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid JSON format",
			})
		}

		// Retrieve the user ID from the token
		userId, err := controller.GetFromToken(c, "ID")
		if err != nil || userId == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User ID not found in token",
			})
		}

		// Parse the user ID as UUID
		uid, err := uuid.Parse(userId.(string))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid User ID format",
			})
		}

		// Assign the parsed user ID to the response struct
		response.UserId = uid

		// Query the database and handle any potential errors
		if err := DB.Where("device_id = ?", response.DeviceId).FirstOrCreate(&response).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Database error",
			})
		}

		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/devices", func(c *fiber.Ctx) error {
		userId, _ := controller.GetFromToken(c, "ID")
		devices := []*structs.Device{}
		DB.Where("user_id = ?", userId).Find(&devices)
		return c.JSON(devices)
	})

	app.Post("/login/verify-google", func(c *fiber.Ctx) error {
		response := new(structs.GoogleTokenResponse)
		json.Unmarshal(c.Body(), response)
		tokenResponse, err := controller.GetAuthDataFromGooglePayload(response.Token, DB)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		return c.JSON(tokenResponse)
	})

	app.Post("/refresh", func(c *fiber.Ctx) error {
		refreshToken := new(structs.RefreshTokenRequest)
		json.Unmarshal(c.Body(), refreshToken)
		return controller.RefreshToken(c, DB, refreshToken.RefreshToken)
	})

	app.Get("/peers", func(c *fiber.Ctx) error {
		userId, _ := controller.GetFromToken(c, "ID")
		members, err := controller.FetchAllMembers(context.Background(), c.Query("networkId"), userId.(string), DB)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		return c.JSON(members)
	})

	app.Post("/nebula/sign-public-key", func(c *fiber.Ctx) error {
		body := struct {
			PublicKey string `json:"public_key"`
			DeviceId  string `json:"device_id"`
		}{}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid request body",
			})
		}

		// TODO, get last public ip
		signedKey, caCert, incomingSite, err := controller.SignPublicKey(body.PublicKey, body.DeviceId, DB)
		if err != nil {
			log.Println(err)
			return c.Status(500).JSON(fiber.Map{
				"error": "Failed to sign public key",
			})
		}

		return c.JSON(fiber.Map{
			"signed_key":    signedKey,
			"ca_cert":       caCert,
			"incoming_site": incomingSite,
		})
	})

	app.Post("/device/send/:id", func(c *fiber.Ctx) error {
		deviceId := c.Params("id")
		userId, _ := controller.GetFromToken(c, "ID")
		SSERequest := new(structs.SSERequest)
		json.Unmarshal(c.Body(), SSERequest)
		device := new(structs.Device)
		DB.Where("device_id = ? AND user_id = ?", SSERequest.UniqueID, userId).First(&device)
		SSEResponse := new(structs.SSEResponse)
		SSEResponse.Type = SSERequest.Type
		SSEResponse.Data = SSERequest.Data
		SSEResponse.Device = *device
		jsonData, _ := json.Marshal(SSEResponse)
		log.Println("Device ID: ", deviceId, "User Id:", userId)
		redisStore.Publish(context.Background(), deviceId, jsonData)
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/device/receive/:id", func(c *fiber.Ctx) error {
		deviceId := c.Params("id")
		log.Println("Device ID: ", deviceId)
		return controller.DeviceSSE(c, redisStore, deviceId)
	})

	cfg := websocket.Config{
		RecoverHandler: func(conn *websocket.Conn) {
			if err := recover(); err != nil {
				conn.WriteJSON(fiber.Map{"customError": "error occurred"})
			}
		},
	}

	app.Get("/stream", websocket.New(func(c *websocket.Conn) {
		// Ensure the connection is closed properly
		defer func() {
			err := c.Close()
			if err != nil {
				log.Println("Error closing connection:", err)
			}
		}()
		// Start a ping-pong loop to keep the connection alive
		go func() {
			ticker := time.NewTicker(15 * time.Second) // Adjust interval as needed
			defer ticker.Stop()

			for range ticker.C {
				if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Println("Error sending ping:", err)
					return
				}
			}
		}()

		controller.Stream(c, redisStore)
	}, cfg))

	app.Get("/proxy/:baseUrl", func(c *fiber.Ctx) error {
		encodedId := c.Params("baseUrl")

		// Step 2: Decode the Base64 ID
		decodedBytes, err := base64.URLEncoding.DecodeString(encodedId)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid Base64 encoding")
		}
		decodedURL := string(decodedBytes)

		// Step 3: Validate the decoded URL
		parsedURL, err := url.Parse(decodedURL)
		if err != nil || !(strings.HasPrefix(parsedURL.Scheme, "http")) {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid URL")
		}
		// Step 4: Redirect to the decoded URL
		return c.Redirect(decodedURL, fiber.StatusTemporaryRedirect)
	})

	log.Fatal(app.Listen(":" + os.Getenv("PORT")))
}
