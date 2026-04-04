package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func StubAnthropicServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"title":  "Test Issue",
			"body":   "Test body from thread",
			"labels": []string{"bug"},
		})
	}))
}
