package controllers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"time"
	"zeroshare-backend/config"
	structs "zeroshare-backend/structs"

	"github.com/gofiber/fiber/v2"
	jtoken "github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
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
	tokenResponse := structs.TokenResponse{
		AuthToken:   t,
		RefresToken: "", // You need to provide a value for RefresToken
	}

	log.Printf("token response is %v", tokenResponse)

	jsonData, err := json.Marshal(tokenResponse)
	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}
	redisStore.Publish(context.Background(), sessionToken, jsonData)

	return c.Status(200).SendString("You may close this window")
}