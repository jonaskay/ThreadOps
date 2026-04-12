package main

import (
	"context"
	"io"
	"log"
	"net/http"
)

type Publisher interface {
	Publish(ctx context.Context, data []byte) error
}

type Verifier interface {
	Verify(signingSecret string, header http.Header, body []byte) error
}

func handleSlackEvent(signingSecret string, v Verifier, pub Publisher) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := v.Verify(signingSecret, r.Header, body); err != nil {
			log.Printf("verify: %v", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if err := pub.Publish(context.Background(), body); err != nil {
			log.Printf("publish: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
