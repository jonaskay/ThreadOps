package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type SlackThread struct {
	Messages []SlackMessage `json:"messages"`
}

type SlackMessage struct {
	User string `json:"user"`
	Text string `json:"text"`
}

type SlackClient interface {
	FetchThread(channel, ts string) (SlackThread, error)
	PostReply(channel, threadTS, issueURL string) error
}

type Issue struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
}

type LLMProvider interface {
	Query(transcript SlackThread) (Issue, error)
}

type GitHubClient interface {
	CreateIssue(issue Issue) (string, error)
}

type PubSubEnvelope struct {
	Message      PubSubMessage `json:"message"`
	Subscription string        `json:"subscription"`
}

type PubSubMessage struct {
	Data      []byte `json:"data"`
	MessageID string `json:"messageId"`
}

type SlackEvent struct {
	Event SlackInnerEvent `json:"event"`
	TS    string          `json:"ts"`
	Type  string          `json:"type"`
}

type SlackInnerEvent struct {
	Channel  string `json:"channel"`
	TS       string `json:"ts"`
	ThreadTS string `json:"thread_ts"`
}

func (e SlackEvent) threadTS() string {
	if e.Event.ThreadTS != "" {
		return e.Event.ThreadTS
	}
	if e.Event.TS != "" {
		return e.Event.TS
	}
	return e.TS
}

func handlePubsubPush(slackClient SlackClient, llmProvider LLMProvider, githubClient GitHubClient) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var envelope PubSubEnvelope
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			log.Printf("read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var event SlackEvent
		if err := json.Unmarshal(envelope.Message.Data, &event); err != nil {
			log.Printf("unmarshal event: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		channel := event.Event.Channel
		if channel == "" {
			log.Printf("missing channel in event")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		threadTS := event.threadTS()
		if threadTS == "" {
			log.Printf("missing thread timestamp in event")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		thread, err := slackClient.FetchThread(channel, threadTS)
		if err != nil {
			log.Printf("fetch thread: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		issue, err := llmProvider.Query(thread)
		if err != nil {
			log.Printf("llm query: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		issueURL, err := githubClient.CreateIssue(issue)
		if err != nil {
			log.Printf("create issue: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := slackClient.PostReply(channel, threadTS, issueURL); err != nil {
			log.Printf("post reply: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
}
