package main

import (
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
)

// TestGenerateAuthSecret tests the generateAuthSecret function
func TestGenerateAuthSecret(t *testing.T) {
	t.Run("normal generation", func(t *testing.T) {
		secret := generateAuthSecret(30)
		assert.Len(t, secret, 30, "Secret should be 30 characters long")
	})

	t.Run("fallback on error", func(t *testing.T) {
		oldReader := rand.Reader
		defer func() { rand.Reader = oldReader }()
		rand.Reader = &errorReader{}

		secret := generateAuthSecret(30)
		assert.Equal(t, "ChhhH9lL3MiTOaEMcguuHiCHVn", secret, "Should return fallback secret")
	})
}

// errorReader is a reader that always returns an error
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("mock read error")
}

// TestReadInput tests the readInput function
func TestReadInput(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	os.Stdin = r

	go func() {
		w.WriteString("test input\n")
		w.Close()
	}()

	result := readInput("prompt: ")
	assert.Equal(t, "test input", result, "Should read input correctly")
}

// TestDownloadFile tests the downloadFile function
func TestDownloadFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test content"))
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile")

	err := downloadFile(ts.URL, filePath)
	assert.NoError(t, err, "Should download file without error")

	content, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.Equal(t, "test content", string(content))
}

// TestUpdateComposeFile tests the updateComposeFile function
func TestUpdateComposeFile(t *testing.T) {
	testYAML := `
services:
  app:
    image: myapp
  jaeger:
    image: jaeger
  clickhouse:
    image: clickhouse
  otel-collector:
    image: otel
  grafana:
    image: grafana
`

	tests := []struct {
		name       string
		setupType  string
		shouldHave []string
		shouldNot  []string
	}{
		{
			name:       "prod setup",
			setupType:  "prod",
			shouldHave: []string{"clickhouse", "otel-collector", "grafana"},
			shouldNot:  []string{"jaeger"},
		},
		{
			name:       "dev setup",
			setupType:  "dev",
			shouldHave: []string{"jaeger"},
			shouldNot:  []string{"clickhouse", "otel-collector", "grafana"},
		},
		{
			name:       "no observability",
			setupType:  "none",
			shouldNot:  []string{"jaeger", "clickhouse", "otel-collector", "grafana"},
			shouldHave: []string{"app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := updateComposeFile(testYAML, tt.setupType)

			var config map[string]interface{}
			err := yaml.Unmarshal([]byte(updated), &config)
			assert.NoError(t, err)

			services := config["services"].(map[string]interface{})

			for _, svc := range tt.shouldHave {
				assert.Contains(t, services, svc, "Should contain service %s", svc)
			}

			for _, svc := range tt.shouldNot {
				assert.NotContains(t, services, svc, "Should not contain service %s", svc)
			}
		})
	}
}
