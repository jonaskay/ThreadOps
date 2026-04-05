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
		want      int
	}{
		{
			name:      "valid request",
			verifier:  &fakeVerifier{err: nil},
			publisher: &fakePublisher{err: nil},
			want:      http.StatusOK,
		},
		{
			name:      "verify fails",
			verifier:  &fakeVerifier{err: errors.New("verify failed")},
			publisher: &fakePublisher{err: nil},
			want:      http.StatusForbidden,
		},
		{
			name:      "publish fails",
			verifier:  &fakeVerifier{err: nil},
			publisher: &fakePublisher{err: errors.New("publish failed")},
			want:      http.StatusInternalServerError,
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {

			body := []byte("foo")
			req := httptest.NewRequest("POST", "/slack/events", bytes.NewReader(body))

			rec := httptest.NewRecorder()
			handleSlackEvent("test-secret", tt.verifier, tt.publisher)(rec, req)

			if rec.Code != tt.want {
				t.Errorf("got %d, want %d", rec.Code, tt.want)
			}

			if !bytes.Equal(tt.publisher.(*fakePublisher).got, body) {
				t.Errorf("published %q, want %q", tt.publisher.(*fakePublisher).got, body)
			}
		})
	}
}
