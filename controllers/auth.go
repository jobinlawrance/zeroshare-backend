package controllers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"
	structs "zeroshare-backend/structs"

	"github.com/gofiber/fiber/v2"
	jtoken "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"
	otel "go.opentelemetry.io/otel"
	attribute "go.opentelemetry.io/otel/attribute"
	codes "go.opentelemetry.io/otel/codes"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	idToken "google.golang.org/api/idtoken"
	"gorm.io/gorm"
)

func SetUpOAuth() *oauth2.Config {
	oauthConf := &oauth2.Config{
		ClientID:     os.Getenv("CLIENT_ID"),     //get google client id from .env file
		ClientSecret: os.Getenv("CLIENT_SECRET"), //get google secret id from .env file
		RedirectURL:  os.Getenv("REDIRECT_URL"),  //get redirect URL from .env file
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
	log.Println("Redirect URL: ", oauthConf.RedirectURL)
	return oauthConf
}

func GetAuthData(c *fiber.Ctx, oauthConf *oauth2.Config, db *gorm.DB, redisStore *redis.Client) error {
	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to exchange token: ")
	}
	token, err := oauthConf.Exchange(context.Background(), code) //get token
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to exchange token: " + err.Error())
	}
	client := oauthConf.Client(context.Background(), token)                      //set client for getting user info like email, name, etc.
	response, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo") //get user info
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to get user info: " + err.Error())
	}

	defer response.Body.Close()
	var user structs.User //user variable

	bytes, err := io.ReadAll(response.Body) //reading response body from client
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error reading response body: " + err.Error())
	}
	err = json.Unmarshal(bytes, &user) //unmarshal user info
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error unmarshal json body " + err.Error())
	}

	db.Where(structs.User{Email: user.Email}).FirstOrCreate(&user)

	log.Printf("user data is %v", user)

	sessionToken := c.Query("state")

	log.Printf("Sending publish message to %s -> %s", sessionToken, user.GoogleID)
	tokenResponse := createToken(user)

	log.Printf("token response is %v", tokenResponse)

	jsonData, err := json.Marshal(tokenResponse)
	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}
	redisStore.Publish(context.Background(), sessionToken, jsonData)

	return c.Status(200).SendString("You may close this window")
}

func createToken(user structs.User) structs.TokenResponse {
	exp := time.Now().Add(time.Hour * 72)

	// Create the JWT claims, which includes the user ID and expiry time
	claims := jtoken.MapClaims{
		"ID":    user.ID,
		"name":  user.Name,
		"email": user.Email,
		"exp":   exp.Unix(),
	}
	// Create token
	jwtToken := jtoken.NewWithClaims(jtoken.SigningMethodHS256, claims)
	// Generate encoded token and send it as response.
	token, err := jwtToken.SignedString([]byte(os.Getenv("AUTH_SECRET")))
	if err != nil {
		log.Fatal(err)
	}

	// Set expiration for refresh token (longer-lived)
	refreshExp := time.Now().Add(time.Hour * 24 * 7)

	// Create the refresh token claims
	refreshClaims := jtoken.MapClaims{
		"ID":  user.ID,
		"exp": refreshExp.Unix(),
	}

	// Create refresh token
	jwtRefreshToken := jtoken.NewWithClaims(jtoken.SigningMethodHS256, refreshClaims)
	refreshToken, err := jwtRefreshToken.SignedString([]byte(os.Getenv("AUTH_SECRET")))
	if err != nil {
		log.Fatal(err)
	}

	return structs.TokenResponse{
		AuthToken:   token,
		RefresToken: refreshToken, // You need to provide a value for RefresToken
	}
}

func GetAuthDataFromGooglePayload(token string, db *gorm.DB) (structs.TokenResponse, error) {
	payload, err := idToken.Validate(context.Background(), token, os.Getenv("CLIENT_ID"))
	if err != nil {
		log.Println(err)
		return structs.TokenResponse{}, err
	}

	user := structs.User{
		Email:         payload.Claims["email"].(string),
		FamilyName:    payload.Claims["family_name"].(string),
		GivenName:     payload.Claims["given_name"].(string),
		Locale:        "",
		Name:          payload.Claims["name"].(string),
		Picture:       payload.Claims["picture"].(string),
		VerifiedEmail: payload.Claims["email_verified"].(bool),
	}

	db.Where(structs.User{Email: user.Email}).FirstOrCreate(&user)

	return createToken(user), nil
}

func SSE(c *fiber.Ctx, redisStore *redis.Client, sessionToken string) error {
	// Start a new span for the SSE connection
	ctx := context.Background()
	tracer := otel.Tracer("zeroshare/controllers")
	ctx, span := tracer.Start(ctx, "SSE.Connection")
	defer span.End()

	// Add attributes to the span
	span.SetAttributes(
		attribute.String("session_token", sessionToken),
		attribute.String("connection_type", "sse"),
	)

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	subscriber := redisStore.Subscribe(context.Background(), sessionToken)

	// Listen for messages on the Redis channel and send them as SSE
	c.Status(fiber.StatusOK).Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {

		defer subscriber.Close()
		for {
			msgCtx, msgSpan := tracer.Start(ctx, "SSE.ReceiveMessage")

			msg, err := subscriber.ReceiveMessage(msgCtx)
			if err != nil {
				msgSpan.SetStatus(codes.Error, err.Error())
				msgSpan.RecordError(err)
				log.Printf("Error receiving message: %v", err)
				msgSpan.End()
				break
			}

			// Send the SSE formatted data
			log.Printf("Sending SSE event to client: %s", sessionToken)
			data := fmt.Sprintf("data: %s\n\n", msg.Payload)

			if _, err := w.WriteString(data); err != nil {
				msgSpan.SetStatus(codes.Error, err.Error())
				msgSpan.RecordError(err)
				log.Printf("Error writing to stream: %v", err)
				msgSpan.End()
				break
			}

			if err := w.Flush(); err != nil {
				msgSpan.SetStatus(codes.Error, err.Error())
				msgSpan.RecordError(err)
				log.Printf("Error flushing stream: %v", err)
				msgSpan.End()
				break
			}

			if _, err := w.WriteString("event: complete\ndata: Authentication completed\n\n"); err != nil {
				log.Printf("Error sending completion event: %v", err)
			}
			w.Flush()
			msgSpan.End()
			return
		}
		return
	}))

	return nil
}

func GetFromToken(c *fiber.Ctx, key string) (interface{}, error) {
	// Extract the JWT token from the Authorization header
	tokenString := c.Get("Authorization")[7:] // Strip "Bearer " prefix
	if tokenString == "" {
		return nil, errors.New("authorization header is missing")
	}

	// Parse the token
	token, _ := jtoken.Parse(tokenString, func(token *jtoken.Token) (interface{}, error) {
		// Ensure that the token's signing method is what we expect
		if _, ok := token.Method.(*jtoken.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return os.Getenv("AUTH_SECRET"), nil
	})

	// Extract the claims from the token (assuming it's a map of claims)
	claims, ok := token.Claims.(jtoken.MapClaims)
	if !ok {
		return nil, errors.New("failed to parse token claims")
	}

	// Check if the user is an admin
	return claims[key], nil
}

func RefreshToken(c *fiber.Ctx, db *gorm.DB, refreshToken string) error {
	// Validate and parse the refresh token
	token, err := jtoken.Parse(refreshToken, func(token *jtoken.Token) (interface{}, error) {
		return []byte(os.Getenv("AUTH_SECRET")), nil
	})
	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid refresh token",
		})
	}

	// Extract claims
	claims, ok := token.Claims.(jtoken.MapClaims)
	if !ok || claims["ID"] == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid token claims",
		})
	}

	// Generate new tokens
	userID, err := uuid.Parse(claims["ID"].(string))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}
	var user structs.User
	err = db.Where(structs.User{ID: userID}).First(&user).Error
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not found",
		})
	}
	if user == (structs.User{}) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(createToken(user))
}
