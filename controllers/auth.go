package controllers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"
	"zeroshare-backend/config"
	structs "zeroshare-backend/structs"

	"github.com/gofiber/fiber/v2"
	jtoken "github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
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

	exp := time.Now().Add(time.Hour * 72)

	// Create the JWT claims, which includes the user ID and expiry time
	claims := jtoken.MapClaims{
		"ID":                     user.ID,
		"name":                   user.Name,
		"email":                  user.Email,
		"exp":                    exp.Unix(),
	}
	// Create token
	jwtToken := jtoken.NewWithClaims(jtoken.SigningMethodHS256, claims)
	// Generate encoded token and send it as response.
	t, err := jwtToken.SignedString([]byte(config.Secret))
	if err != nil {
		log.Fatal(err)
	}

	if user.ZtNetworkId == "" {
		nwid, err := CreateNewZTNetwork(context.Background())
		if err != nil {
			log.Fatal(err)
		}
		user.ZtNetworkId = nwid
		db.Where(structs.User{Email: user.Email}).Updates(structs.User{ZtNetworkId: user.ZtNetworkId})
	}
	
	tokenResponse := structs.TokenResponse{
		AuthToken:   t,
		RefresToken: "", // You need to provide a value for RefresToken
		ZtNetworkId: user.ZtNetworkId,
	}

	log.Printf("token response is %v", tokenResponse)

	jsonData, err := json.Marshal(tokenResponse)
	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}
	redisStore.Publish(context.Background(), sessionToken, jsonData)

	return c.Status(200).SendString("You may close this window")
}

func SSE(c *fiber.Ctx, redisStore *redis.Client, sessionToken string) error {

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	subscriber := redisStore.Subscribe(context.Background(), sessionToken)

	// Listen for messages on the Redis channel and send them as SSE
	c.Status(fiber.StatusOK).Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		for {
			msg, err := subscriber.ReceiveMessage(context.Background())
			if err != nil {
				log.Printf("Error receiving message: %v", err)
				break
			}

			// Send the SSE formatted data
			log.Printf("Sending SSE event to client: %s", sessionToken)
			data := fmt.Sprintf("data: %s\n\n", msg.Payload)

			log.Printf("Payload: %s", msg.Payload)

			// Write data to the stream
			if _, err := w.WriteString(data); err != nil {
				log.Printf("Error writing to stream: %v", err)
				break
			}

			// Flush the response to send the data immediately
			if err := w.Flush(); err != nil {
				log.Printf("Error flushing stream: %v", err)
				break
			}
		}
	}))

	return nil
}