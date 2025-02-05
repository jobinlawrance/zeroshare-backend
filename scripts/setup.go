package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/urfave/cli/v2"
)

type Tag struct {
	Name string `json:"name"`
}

func generateAuthSecret(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "ChhhH9lL3MiTOaEMcguuHiCHVn" // fallback
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

func main() {
	app := &cli.App{
		Name:  "ZeroShare Backend Setup",
		Usage: "Setup zeroshare backend",
		Action: func(c *cli.Context) error {
			// Create directory
			err := os.MkdirAll("zeroshare-backend", 0755)
			if err != nil {
				return fmt.Errorf("failed to create directory: %v", err)
			}

			// Change directory
			err = os.Chdir("zeroshare-backend")
			if err != nil {
				return fmt.Errorf("failed to change directory: %v", err)
			}

			// Download compose file
			resp, err := http.Get("https://raw.githubusercontent.com/jobinlawrance/zeroshare-backend/main/docker-compose.yml")
			if err != nil {
				return fmt.Errorf("failed to download compose file: %v", err)
			}
			defer resp.Body.Close()

			composeContent, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read compose file: %v", err)
			}

			// Get latest tag
			tagResp, err := http.Get("https://api.github.com/repos/jobinlawrance/zeroshare-backend/tags")
			if err != nil {
				return fmt.Errorf("failed to get tags: %v", err)
			}
			defer tagResp.Body.Close()

			var tags []Tag
			if err := json.NewDecoder(tagResp.Body).Decode(&tags); err != nil {
				return fmt.Errorf("failed to decode tags: %v", err)
			}

			if len(tags) == 0 {
				return fmt.Errorf("no tags found")
			}

			version := strings.TrimPrefix(tags[0].Name, "v")

			// Update compose file with correct version
			re := regexp.MustCompile(`image: jobinlawrance/zeroshare-backend:[^\s]*`)
			updatedCompose := re.ReplaceAllString(
				string(composeContent),
				fmt.Sprintf("image: ghcr.io/jobinlawrance/zeroshare-backend:%s", version),
			)

			err = os.WriteFile("docker-compose.yml", []byte(updatedCompose), 0644)
			if err != nil {
				return fmt.Errorf("failed to write compose file: %v", err)
			}

			// Get user input
			fmt.Print("Enter Google Client ID: ")
			var clientID string
			fmt.Scanln(&clientID)

			fmt.Print("Enter Google Client Secret: ")
			var clientSecret string
			fmt.Scanln(&clientSecret)

			fmt.Print("Enter Redirect URI: ")
			var redirectURI string
			fmt.Scanln(&redirectURI)

			// Check if nebula is installed
			cmd := exec.Command("nebula", "--version")
			if err := cmd.Run(); err != nil {
				fmt.Println("Nebula not found. Installing...")
				install_nebula()
				fmt.Println("Nebula installed successfully!")
			}

			// Generate auth secret
			authSecret := generateAuthSecret(30)

			// Get system timezone
			timezoneName := "UTC" // default fallback
			if tzData, err := os.ReadFile("/etc/timezone"); err == nil {
				timezoneName = strings.TrimSpace(string(tzData))
			} else if link, err := os.Readlink("/etc/localtime"); err == nil {
				if parts := strings.Split(link, "zoneinfo/"); len(parts) > 1 {
					timezoneName = parts[1]
				}
			}

			// Create .env file
			envContent := fmt.Sprintf(`PORT=4000
DB_HOST=localhost       
DB_USER=postgres
DB_PASSWORD=root
DB_NAME=zeroshare
DB_PORT=5432
DB_SSLMODE=disable
DB_TIMEZONE=%s
REDIS_HOST=localhost
REDIS_PORT=6379
CLIENT_ID=%s
CLIENT_SECRET=%s
REDIRECT_URL=%s
AUTH_SECRET=%s
`, timezoneName, clientID, clientSecret, redirectURI, authSecret)

			err = os.WriteFile(".env", []byte(envContent), 0644)
			if err != nil {
				return fmt.Errorf("failed to write .env file: %v", err)
			}

			// Run docker compose
			cmd = exec.Command("docker", "compose", "up", "-d")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to start docker compose: %v", err)
			}

			fmt.Println("Setup completed successfully!")
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
