package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	threadopsv1 "github.com/jonaskay/threadops/internal/gen/threadops/v1"
	"google.golang.org/protobuf/proto"
)

const validSlackPayload = `{
	"token": "test-token",
	"team_id": "T123",
	"api_app_id": "A123",
	"event": {
		"type": "app_mention",
		"user": "U123",
		"text": "<@U456> create an issue",
		"ts": "1234567890.000100",
		"channel": "C123",
		"event_ts": "1234567890.000100"
	},
	"type": "event_callback",
	"event_id": "Ev123",
	"event_time": 1234567890,
	"authorizations": [{"team_id": "T123", "user_id": "U456", "is_bot": true, "is_enterprise_install": false}]
}`

type fakePublisher struct {
	got proto.Message
	err error
}

func (f *fakePublisher) Publish(ctx context.Context, msg proto.Message) error {
	f.got = msg
	return f.err
}

type fakeVerifier struct {
	err error
}

func (f *fakeVerifier) Verify(signingSecret string, header http.Header, body []byte) error {
	return f.err
}

func TestHandleSlackEvent(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		verifier  Verifier
		publisher Publisher
		wantCode  int
	}{
		{
			name:      "valid request",
			body:      validSlackPayload,
			verifier:  &fakeVerifier{err: nil},
			publisher: &fakePublisher{err: nil},
			wantCode:  http.StatusOK,
		},
		{
			name:      "verify fails",
			body:      validSlackPayload,
			verifier:  &fakeVerifier{err: errors.New("verify failed")},
			publisher: &fakePublisher{err: nil},
			wantCode:  http.StatusForbidden,
		},
		{
			name:      "publish fails",
			body:      validSlackPayload,
			verifier:  &fakeVerifier{err: nil},
			publisher: &fakePublisher{err: errors.New("publish failed")},
			wantCode:  http.StatusInternalServerError,
		},
		{
			name:      "malformed JSON",
			body:      `{not valid json`,
			verifier:  &fakeVerifier{err: nil},
			publisher: &fakePublisher{err: nil},
			wantCode:  http.StatusBadRequest,
		},
		{
			name:      "url_verification challenge",
			body:      `{"type":"url_verification","challenge":"abc123","token":"test-token"}`,
			verifier:  &fakeVerifier{err: nil},
			publisher: &fakePublisher{err: nil},
			wantCode:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/slack/events", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			handleSlackEvent("test-secret", tt.verifier, tt.publisher)(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got %d, want %d", rec.Code, tt.wantCode)
			}
		})
	}
}

func TestHandleSlackEventURLVerification(t *testing.T) {
	body := `{"type":"url_verification","challenge":"abc123","token":"test-token"}`
	req := httptest.NewRequest("POST", "/slack/events", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handleSlackEvent("test-secret", &fakeVerifier{}, &fakePublisher{})(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["challenge"] != "abc123" {
		t.Errorf("challenge = %q, want %q", resp["challenge"], "abc123")
	}
}

func TestHandleSlackEventParsesFields(t *testing.T) {
	pub := &fakePublisher{}
	req := httptest.NewRequest("POST", "/slack/events", bytes.NewBufferString(validSlackPayload))
	rec := httptest.NewRecorder()
	handleSlackEvent("test-secret", &fakeVerifier{}, pub)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	event, ok := pub.got.(*threadopsv1.SlackEvent)
	if !ok || event == nil {
		t.Fatal("published message is not a *SlackEvent")
	}
	if event.TeamId != "T123" {
		t.Errorf("TeamId = %q, want %q", event.TeamId, "T123")
	}
	if event.Event.GetType() != "app_mention" {
		t.Errorf("Event.Type = %q, want %q", event.Event.GetType(), "app_mention")
	}
}
