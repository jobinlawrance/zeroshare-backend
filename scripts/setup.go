package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/schollz/progressbar/v3"
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
			re := regexp.MustCompile(`image: ghcr.io/jobinlawrance/zeroshare-backend:[^\s]*`)
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
				if err := installNebula(); err != nil {
					return fmt.Errorf("failed to install nebula: %v", err)
				}
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

type GithubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type GithubReleaseResponse struct {
	Name   string        `json:"name"`
	Assets []GithubAsset `json:"assets"`
}

// Modify the InstallNebula function to be callable
func installNebula() error {
	// Detect OS and Architecture
	osName := detectOS()
	arch := detectArch()

	fmt.Printf("Detected OS: %s, Architecture: %s\n", osName, arch)

	// Fetch latest release
	fmt.Println("Fetching latest release from GitHub...")
	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch latest release: %w", err)
	}
	fmt.Printf("Latest release: %s\n", release.Name)

	// Find matching asset
	asset, err := findMatchingAsset(release, osName, arch)
	if err != nil {
		return err
	}
	fmt.Printf("Downloading: %s\n", asset.Name)

	// Download the file
	downloadedFile, err := downloadFile(asset.BrowserDownloadURL, asset.Name)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer os.Remove(downloadedFile)

	fmt.Println("Download completed. Extracting...")
	// Extract and place binary
	tempDir, err := os.MkdirTemp("", "nebula_install")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	err = extractFile(downloadedFile, tempDir)
	if err != nil {
		return fmt.Errorf("failed to extract file: %w", err)
	}

	// makeFilesExecutable walks through the directory and makes all files executable.
	err = makeFilesExecutable(tempDir)
	if err != nil {
		return fmt.Errorf("failed to make files executable: %w", err)
	}

	// Move binary to /usr/local/bin
	fmt.Println("Moving binary to /usr/local/bin...")
	err = moveToUsrLocalBin(tempDir)
	if err != nil {
		return err
	}

	fmt.Println("Installation complete.")
	return nil
}

func makeFilesExecutable(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			if err := os.Chmod(path, 0755); err != nil {
				return err
			}
		}
		return nil
	})
}

func detectOS() string {
	return runtime.GOOS
}

func detectArch() string {
	return runtime.GOARCH
}

func fetchLatestRelease() (*GithubReleaseResponse, error) {
	resp, err := http.Get("https://api.github.com/repos/slackhq/nebula/releases/latest")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var release GithubReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func findMatchingAsset(release *GithubReleaseResponse, osName, arch string) (*GithubAsset, error) {
	var osAssets []GithubAsset
	for _, asset := range release.Assets {
		if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(osName)) {
			osAssets = append(osAssets, asset)
		}
	}
	// If no assets match the OS, return an error
	if len(osAssets) == 0 {
		return nil, fmt.Errorf("no matching asset found for OS: %s", osName)
	}

	// If only one asset matches the OS, return it
	if len(osAssets) == 1 {
		return &osAssets[0], nil
	}

	// Otherwise, filter by architecture
	for _, asset := range osAssets {
		if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(arch)) {
			return &asset, nil
		}
	}

	// If no assets match both OS and architecture, return an error
	return nil, fmt.Errorf("no matching asset found for OS: %s and Architecture: %s", osName, arch)
}

func downloadFile(url, filename string) (string, error) {
	// Make the HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file: %w", err)
	}
	defer resp.Body.Close()

	// Get the content length (if provided by the server)
	contentLength, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil || contentLength <= 0 {
		fmt.Println("Unable to determine file size for progress bar. Proceeding without it.")
		contentLength = -1 // Fallback to unknown content length
	}

	// Create the file on disk
	out, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Set up the progress bar
	bar := progressbar.NewOptions(
		contentLength,
		progressbar.OptionSetDescription("Downloading"),
		progressbar.OptionSetTheme(progressbar.ThemeDefault),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(40),
	)

	// Copy the response body to the file while updating the progress bar
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return filename, nil
}

func extractFile(filePath, destDir string) error {
	if strings.HasSuffix(filePath, ".tar.gz") {
		return extractTarGz(filePath, destDir)
	} else if strings.HasSuffix(filePath, ".zip") {
		return extractZip(filePath, destDir)
	}
	return fmt.Errorf("unsupported file format: %s", filePath)
}

func extractTarGz(filePath, destDir string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(destDir, header.Name)
		if header.Typeflag == tar.TypeDir {
			os.MkdirAll(targetPath, 0755)
		} else {
			os.MkdirAll(filepath.Dir(targetPath), 0755)
			outFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			_, err = io.Copy(outFile, tarReader)
			outFile.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func extractZip(filePath, destDir string) error {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		targetPath := filepath.Join(destDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, 0755)
		} else {
			os.MkdirAll(filepath.Dir(targetPath), 0755)
			outFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			rc, err := f.Open()
			if err != nil {
				return err
			}
			_, err = io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func moveToUsrLocalBin(sourceDir string) error {
	cmd := exec.Command("sudo", "cp", "-r", sourceDir+"/.", "/usr/local/bin/")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
