package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	threadopsv1 "github.com/jonaskay/threadops/internal/gen/threadops/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Publisher interface {
	Publish(ctx context.Context, msg proto.Message) error
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

		var peek struct {
			Type      string `json:"type"`
			Challenge string `json:"challenge"`
		}
		if err := json.Unmarshal(body, &peek); err == nil && peek.Type == "url_verification" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"challenge": peek.Challenge})
			return
		}

		var event threadopsv1.SlackEvent
		unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: true}
		if err := unmarshaler.Unmarshal(body, &event); err != nil {
			log.Printf("unmarshal: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := pub.Publish(context.Background(), &event); err != nil {
			log.Printf("publish: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
