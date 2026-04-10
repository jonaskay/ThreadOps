package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePublisher struct {
	got []byte
	err error
}

func (f *fakePublisher) Publish(ctx context.Context, data []byte) error {
	f.got = data

	return f.err
}

type fakeVerifier struct {
	err error
}

func (f *fakeVerifier) Verify(signingSecret string, header http.Header, body []byte) error {
	return f.err
}
func TestHandleSlackEvent(t *testing.T) {
	test := []struct {
		name      string
		verifier  Verifier
		publisher Publisher
		wantCode  int
		wantBody  []byte
	}{
		{
			name:      "valid request",
			verifier:  &fakeVerifier{err: nil},
			publisher: &fakePublisher{err: nil},
			wantCode:  http.StatusOK,
			wantBody:  []byte("foo"),
		},
		{
			name:      "verify fails",
			verifier:  &fakeVerifier{err: errors.New("verify failed")},
			publisher: &fakePublisher{err: nil},
			wantCode:  http.StatusForbidden,
			wantBody:  []byte(""),
		},
		{
			name:      "publish fails",
			verifier:  &fakeVerifier{err: nil},
			publisher: &fakePublisher{err: errors.New("publish failed")},
			wantCode:  http.StatusInternalServerError,
			wantBody:  []byte("foo"),
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {

			body := []byte("foo")
			req := httptest.NewRequest("POST", "/slack/events", bytes.NewReader(body))

			rec := httptest.NewRecorder()
			handleSlackEvent("test-secret", tt.verifier, tt.publisher)(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got %d, want %d", rec.Code, tt.wantCode)
			}

			if !bytes.Equal(tt.publisher.(*fakePublisher).got, tt.wantBody) {
				t.Errorf("published %q, want %q", tt.publisher.(*fakePublisher).got, body)
			}
		})
	}
}
