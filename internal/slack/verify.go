package internal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func Verify(signingSecret string, header http.Header, body []byte) error {
	timestampStr := header.Get("X-Slack-Request-Timestamp")
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	if time.Since(time.Unix(timestamp, 0)) > 5*time.Minute {
		return fmt.Errorf("timestamp too old")
	}

	expected := validSignature(signingSecret, timestampStr, body)

	sig := header.Get("X-Slack-Signature")
	if hmac.Equal([]byte(sig), []byte(expected)) {
		return nil
	}

	return fmt.Errorf("invalid signature")
}

func validSignature(secret string, timestamp string, body []byte) string {
	basestring := "v0:" + timestamp + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(basestring))

	return "v0=" + hex.EncodeToString(mac.Sum(nil))

}
