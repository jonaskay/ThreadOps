package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jonaskay/threadops/internal/pubsub"
	"github.com/jonaskay/threadops/internal/slack"
)

type VerifierFunc func(string, http.Header, []byte) error

func (f VerifierFunc) Verify(signingSecret string, header http.Header, body []byte) error {
	return f(signingSecret, header, body)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	signingSecret := os.Getenv("SLACK_SIGNING_SECRET")
	if signingSecret == "" {
		log.Fatal("SLACK_SIGNING_SECRET is not set")
	}
	projectID := os.Getenv("PROJECT_ID")
	if projectID == "" {
		log.Fatal("PROJECT_ID is not set")
	}
	topicID := os.Getenv("PUBSUB_TOPIC")
	if topicID == "" {
		log.Fatal("PUBSUB_TOPIC is not set")
	}

	verifier := VerifierFunc(slack.Verify)
	pub := pubsub.NewPublisher(context.Background(), projectID, topicID)

	http.HandleFunc("/slack/events", handleSlackEvent(signingSecret, verifier, pub))

	fmt.Printf("webhook listening on :%s\n", port)
	http.ListenAndServe(":"+port, nil)
}
