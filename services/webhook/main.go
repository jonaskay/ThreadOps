package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	internal "github.com/jonaskay/threadops/internal/slack"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	signingSecret := os.Getenv("SLACK_SIGNING_SECRET")
	if signingSecret == "" {
		log.Fatal("SLACK_SIGNING_SECRET is not set")
	}

	http.HandleFunc("/slack/events", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := internal.Verify(signingSecret, r.Header, body); err != nil {
			log.Printf("verify: %v", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	fmt.Printf("webhook listening on :%s\n", port)
	http.ListenAndServe(":"+port, nil)
}
