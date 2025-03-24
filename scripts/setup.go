package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"

	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"

	"strings"

	"github.com/goccy/go-yaml"
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
			resp, err := http.Get("https://raw.githubusercontent.com/jobinlawrance/zeroshare-backend/refs/heads/main/docker-compose.uncommented.yml")
			if err != nil {
				return fmt.Errorf("failed to download compose file: %v", err)
			}
			defer resp.Body.Close()

			composeContent, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read compose file: %v", err)
			}

			// Get user input using the new function
			orgName := readInput("Enter Organization Name: ")
			clientID := readInput("Enter Google Client ID: ")
			clientSecret := readInput("Enter Google Client Secret: ")

			// Ask about observability
			enableObservability := strings.ToLower(readInput("Do you want to enable Observability (Logs, Traces & Metrics)? (yes/no): "))

			otelMetrics := "false"
			otelLogs := "false"
			otelTracing := "false"
			otelEndpoint := "localhost"

			if enableObservability == "yes" || enableObservability == "y" {
				otelTracing = "true"

				// Ask about setup type
				setupType := strings.ToLower(readInput("Do you want a permanent (prod) or temporary (dev) setup? (prod/dev): "))

				// Create otel directory structure for prod setup
				if setupType == "prod" {
					otelEndpoint = "otel-collector"
					otelMetrics = "true"
					otelLogs = "true"
					err := os.MkdirAll("otel/clickhouse-init", 0755)
					if err != nil {
						return fmt.Errorf("failed to create otel directories: %v", err)
					}

					// Download and save otel configuration files
					files := map[string]string{
						"otel/datasource.yaml":             "https://github.com/jobinlawrance/zeroshare-backend/raw/refs/heads/main/otel/datasource.yaml",
						"otel/grafana.ini":                 "https://github.com/jobinlawrance/zeroshare-backend/raw/refs/heads/main/otel/grafana.ini",
						"otel/otel-collector-config.yaml":  "https://github.com/jobinlawrance/zeroshare-backend/raw/refs/heads/main/otel/otel-collector-config.yaml",
						"otel/clickhouse-init/init-db.sql": "https://github.com/jobinlawrance/zeroshare-backend/raw/refs/heads/main/otel/clickhouse-init/init-db.sql",
					}

					for filePath, url := range files {
						if err := downloadFile(url, filePath); err != nil {
							return fmt.Errorf("failed to download %s: %v", filePath, err)
						}
					}
				} else {
					otelEndpoint = "jaeger"
				}

				// Update compose file based on setup type
				updatedContent := updateComposeFile(string(composeContent), setupType)
				composeContent = []byte(updatedContent)
			} else {
				// Comment out all observability services if not enabled
				updatedContent := updateComposeFile(string(composeContent), "none")
				composeContent = []byte(updatedContent)
			}

			// Update the compose file
			err = os.WriteFile("docker-compose.yml", []byte(composeContent), 0644)
			if err != nil {
				return fmt.Errorf("failed to write compose file: %v", err)
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

			// Create .env file with updated OTEL settings
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
REDIRECT_URL=http://localhost:4000/auth/google/callback
AUTH_SECRET=%s
OTEL_METRICS_ENABLED=%s
OTEL_LOGS_ENABLED=%s
OTEL_TRACING_ENABLED=%s
OTEL_EXPORTER_OTLP_ENDPOINT=%s:4317
NEBULA_CERT_PATH=./bin/nebula-cert
`, timezoneName, clientID, clientSecret, authSecret, otelMetrics, otelLogs, otelTracing, otelEndpoint)

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

// New helper functions
func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, content, 0644)
}

func updateComposeFile(content, setupType string) string {
	var composeConfig map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &composeConfig); err != nil {
		return content
	}

	services := composeConfig["services"].(map[string]interface{})
	observabilityServices := []string{"jaeger", "clickhouse", "otel-collector", "grafana"}

	// Create a copy of the original compose file
	var baseConfig map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &baseConfig); err != nil {
		return content
	}

	// Handle observability services based on setup type
	switch setupType {
	case "prod":
		// Remove jaeger, keep other observability services
		delete(services, "jaeger")
	case "dev":
		// Keep only jaeger, remove other observability services
		for _, service := range observabilityServices {
			if service != "jaeger" {
				delete(services, service)
			}
		}
	case "none":
		// Remove all observability services
		for _, service := range observabilityServices {
			delete(services, service)
		}
	}

	// Update the services in the config
	composeConfig["services"] = services

	updatedContent, err := yaml.Marshal(composeConfig)
	if err != nil {
		return content
	}

	return string(updatedContent)
}
