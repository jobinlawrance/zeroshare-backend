package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)


type Response struct {
	Nwid string `json:"nwid"`
}

func CreateNewZTNetwork(ctx context.Context) (string, error) {
	nodeId := os.Getenv("ZU_NODE_ID")
	url := fmt.Sprintf("https://zero-controller.jkbx.live/controller/network/%s______", nodeId)

	log.Printf("URL: %s", url)

	// Create an HTTP client with a timeout
	client := &http.Client{Timeout: 10 * time.Second}

	jsonBody := []byte(`{}`)
    bodyReader := bytes.NewReader(jsonBody)

	// Create a request with the given context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
	req.Header.Add("X-ZT1-AUTH", os.Getenv("ZU_TOKEN"))

	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check for a successful status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Decode the JSON response
	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	return result.Nwid, nil
}