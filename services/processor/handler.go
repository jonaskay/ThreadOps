package main

import (
	"io"
	"log"
	"net/http"
)

type SlackClient interface {
	FetchThread() error
	PostReply() error
}

type TranscriptParser interface {
	Parse() error
}

type LLMProvider interface {
	Query() error
}

type GitHubClient interface {
	CreateIssue() error
}

func handlePubsubPush(slackClient SlackClient, transcriptParser TranscriptParser, llmProvider LLMProvider, githubClient GitHubClient, threadTs string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
