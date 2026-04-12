package e2e

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	testSigningSecret = "test-signing-secret"
	fakeIssueURL      = "https://github.com/testowner/testrepo/issues/42"
)

func TestFullPipeline(t *testing.T) {
	env := newEnv(t)

	// Construct Slack event payload.
	payload := map[string]interface{}{
		"type": "event_callback",
		"event": map[string]interface{}{
			"type":      "app_mention",
			"channel":   "C12345",
			"thread_ts": "1234567890.123456",
			"text":      "<@U0001> create an issue from this thread",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	// Compute HMAC signature.
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sigBasestring := "v0:" + timestamp + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(testSigningSecret))
	mac.Write([]byte(sigBasestring))
	signature := "v0=" + fmt.Sprintf("%x", mac.Sum(nil))

	// POST to webhook.
	webhookURL := fmt.Sprintf("http://localhost:%d/slack/events", env.WebhookPort)
	req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", signature)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST to webhook: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("webhook returned %d: %s", resp.StatusCode, respBody)
	}

	// Assert Slack thread fetched.
	select {
	case <-env.SlackThreadCh:
		// Success.
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for Slack thread fetch")
	}

	// Assert LLM call received.
	select {
	case <-env.LLMCallCh:
		// Success.
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for LLM call")
	}

	// Assert GitHub issue created.
	select {
	case issue := <-env.GitHubIssueCh:
		if issue.Title == "" {
			t.Error("GitHub issue title is empty")
		}
		if issue.Body == "" {
			t.Error("GitHub issue body is empty")
		}
		if len(issue.Labels) == 0 {
			t.Error("GitHub issue has no labels")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for GitHub issue creation")
	}

	// Assert Slack reply posted.
	select {
	case reply := <-env.SlackReplyCh:
		if !strings.Contains(reply.Text, fakeIssueURL) {
			t.Errorf("Slack reply does not contain issue URL %q: got %q", fakeIssueURL, reply.Text)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for Slack reply")
	}
}
