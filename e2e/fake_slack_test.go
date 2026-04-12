package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type SlackReply struct {
	Channel  string `json:"channel"`
	ThreadTS string `json:"thread_ts"`
	Text     string `json:"text"`
}

func FakeSlackServer(t *testing.T, threadCh chan<- string, replyCh chan<- SlackReply) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/conversations.replies":
			threadCh <- "fetched"
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"messages": []map[string]interface{}{
					{"user": "U1001", "text": "We need to track this bug", "ts": "1234567890.000001"},
					{"user": "U1002", "text": "Agreed, it keeps happening in prod", "ts": "1234567890.000002"},
					{"user": "U1003", "text": "<@U0001> create an issue from this thread", "ts": "1234567890.123456"},
				},
			})

		case "/api/chat.postMessage":
			var reply SlackReply
			if r.Header.Get("Content-Type") == "application/json" {
				json.NewDecoder(r.Body).Decode(&reply)
			} else {
				r.ParseForm()
				reply.Channel = r.FormValue("channel")
				reply.ThreadTS = r.FormValue("thread_ts")
				reply.Text = r.FormValue("text")
			}
			replyCh <- reply
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})

		default:
			http.NotFound(w, r)
		}
	}))
}
