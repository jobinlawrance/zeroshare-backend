package main

import (
	"bufio"
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

func readInput(prompt string) string {
	fmt.Print(prompt)
	var input string
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		input = scanner.Text()
	}
	return strings.TrimSpace(input)
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
			re := regexp.MustCompile(`image: ghcr.io/jobinlawrance/zeroshare-backend:[^\s]*`)
			updatedCompose := re.ReplaceAllString(
				string(composeContent),
				fmt.Sprintf("image: ghcr.io/jobinlawrance/zeroshare-backend:%s", version),
			)

			err = os.WriteFile("docker-compose.yml", []byte(updatedCompose), 0644)
			if err != nil {
				return fmt.Errorf("failed to write compose file: %v", err)
			}

			// Get user input using the new function
			orgName := readInput("Enter Organization Name: ")
			clientID := readInput("Enter Google Client ID: ")
			clientSecret := readInput("Enter Google Client Secret: ")
			redirectURI := readInput("Enter Redirect URI: ")

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
DB_HOST=db       
DB_USER=postgres
DB_PASSWORD=root
DB_NAME=zeroshare
DB_PORT=5432
DB_SSLMODE=disable
DB_TIMEZONE=%s
REDIS_HOST=redis
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

			// Create scripts directory
			err = os.MkdirAll("scripts", 0755)
			if err != nil {
				return fmt.Errorf("failed to create scripts directory: %v", err)
			}

			// Download start-lighthouse.sh
			resp, err = http.Get("https://github.com/jobinlawrance/zeroshare-backend/raw/refs/heads/main/scripts/start-lighthouse.sh")
			if err != nil {
				return fmt.Errorf("failed to download start-lighthouse script: %v", err)
			}
			defer resp.Body.Close()

			scriptContent, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read script content: %v", err)
			}

			// Replace organization name in the script
			updatedScript := strings.Replace(
				string(scriptContent),
				"'ZeroShare, Inc'",
				fmt.Sprintf("'%s'", orgName),
				-1,
			)

			err = os.WriteFile("scripts/start-lighthouse.sh", []byte(updatedScript), 0755)
			if err != nil {
				return fmt.Errorf("failed to write start-lighthouse script: %v", err)
			}

			// Create config directory
			err = os.MkdirAll("config", 0755)
			if err != nil {
				return fmt.Errorf("failed to create config directory: %v", err)
			}

			// Download config.yml
			resp, err = http.Get("https://github.com/jobinlawrance/zeroshare-backend/raw/refs/heads/main/config/config.yml")
			if err != nil {
				return fmt.Errorf("failed to download config file: %v", err)
			}
			defer resp.Body.Close()

			configContent, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read config content: %v", err)
			}

			err = os.WriteFile("config/config.yml", configContent, 0644)
			if err != nil {
				return fmt.Errorf("failed to write config file: %v", err)
			}

			// Run docker compose
			cmd := exec.Command("docker", "compose", "up", "-d")
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

type GithubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type GithubReleaseResponse struct {
	Name   string        `json:"name"`
	Assets []GithubAsset `json:"assets"`
}
