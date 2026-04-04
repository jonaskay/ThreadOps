package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type AnthropicMessage struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type AnthropicMessageRequest struct {
	MaxTokens int                `json:"max_tokens"`
	Messages  []AnthropicMessage `json:"messages"`
	Model     string             `json:"model"`
}

func FakeAnthropicServer(t *testing.T, ch chan<- AnthropicMessageRequest) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/v1/messages") {
			http.NotFound(w, r)
			return
		}

		auth := r.Header.Get("X-Api-Key")
		if auth != testAnthropicAPIKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"text": "Test Issue",
					"type": "text",
				},
			},
		})
	}))
}
