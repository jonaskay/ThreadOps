package internal

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestVerify(t *testing.T) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := validSignature("test-secret", timestamp, []byte(`"{type":"event_callback"}`))

	test := []struct {
		name    string
		header  http.Header
		body    []byte
		wantErr bool
	}{
		{
			name: "valid signature",
			header: http.Header{
				"X-Slack-Request-Timestamp": []string{timestamp},
				"X-Slack-Signature":         []string{signature},
			},
			body:    []byte(`"{type":"event_callback"}`),
			wantErr: false,
		},
		{
			name: "missing signature header",
			header: http.Header{
				"X-Slack-Request-Timestamp": []string{timestamp},
			},
			body:    []byte(`"{type":"event_callback"}`),
			wantErr: true,
		},
		{
			name: "wrong signature",
			header: http.Header{
				"X-Slack-Request-Timestamp": []string{timestamp},
				"X-Slack-Signature":         []string{"v0=abc123"},
			},
			body:    []byte(`"{type":"event_callback"}`),
			wantErr: true,
		},
		{
			name: "missing timestamp header",
			header: http.Header{
				"X-Slack-Signature": []string{signature},
			},
			body:    []byte(`"{type":"event_callback"}`),
			wantErr: true,
		},
		{
			name: "timestamp too old",
			header: http.Header{
				"X-Slack-Request-Timestamp": []string{fmt.Sprintf("%d", time.Now().Unix()-6*60)},
				"X-Slack-Signature":         []string{signature},
			},
			body:    []byte(`"{type":"event_callback"}`),
			wantErr: true,
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			err := Verify("test-secret", tt.header, tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("Verify() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
