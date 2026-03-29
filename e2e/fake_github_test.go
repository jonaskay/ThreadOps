package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type GitHubIssueRequest struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
}

func FakeGitHubServer(t *testing.T, ch chan<- GitHubIssueRequest) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/issues") {
			http.NotFound(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth != "token "+testGitHubToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req GitHubIssueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ch <- req

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"html_url": %q}`, fakeIssueURL)
	}))
}
