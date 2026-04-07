package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeSlackClient struct {
	fetchThreadGot []byte
	fetchThreadErr error
	postReplyGot   []byte
	postReplyErr   error
}

func (f *fakeSlackClient) FetchThread() error {
	f.fetchThreadGot = []byte("fetch thread")

	return f.fetchThreadErr
}
func (f *fakeSlackClient) PostReply() error {
	f.postReplyGot = []byte("post reply")

	return f.postReplyErr
}

type fakeTranscriptParser struct {
	got []byte
	err error
}

func (f *fakeTranscriptParser) Parse() error {
	f.got = []byte("parse")

	return f.err
}

type fakeLLMProvider struct {
	got []byte
	err error
}

func (f *fakeLLMProvider) Query() error {
	f.got = []byte("query")
	return f.err
}

type fakeGitHubClient struct {
	got []byte
	err error
}

func (f *fakeGitHubClient) CreateIssue() error {
	f.got = []byte("create issue")
	return f.err
}

func TestHandlePubsubPush(t *testing.T) {
	test := []struct {
		name             string
		slackClient      SlackClient
		transcriptParser TranscriptParser
		llmProvider      LLMProvider
		githubClient     GitHubClient
		want             int
	}{
		{
			name:             "valid request",
			slackClient:      &fakeSlackClient{},
			transcriptParser: &fakeTranscriptParser{},
			llmProvider:      &fakeLLMProvider{},
			githubClient:     &fakeGitHubClient{},
			want:             http.StatusOK,
		},
		{
			name:             "fetch conversation fails",
			slackClient:      &fakeSlackClient{},
			transcriptParser: &fakeTranscriptParser{},
			llmProvider:      &fakeLLMProvider{},
			githubClient:     &fakeGitHubClient{},
			want:             http.StatusInternalServerError,
		},
		{
			name:             "conversation parsing fails",
			slackClient:      &fakeSlackClient{},
			transcriptParser: &fakeTranscriptParser{},
			llmProvider:      &fakeLLMProvider{},
			githubClient:     &fakeGitHubClient{},
			want:             http.StatusInternalServerError,
		},
		{
			name:             "llm call fails",
			slackClient:      &fakeSlackClient{},
			transcriptParser: &fakeTranscriptParser{},
			llmProvider:      &fakeLLMProvider{},
			githubClient:     &fakeGitHubClient{},
			want:             http.StatusInternalServerError,
		},
		{
			name:             "issue creation fails",
			slackClient:      &fakeSlackClient{},
			transcriptParser: &fakeTranscriptParser{},
			llmProvider:      &fakeLLMProvider{},
			githubClient:     &fakeGitHubClient{},
			want:             http.StatusInternalServerError,
		},
		{
			name:             "slack reply fails",
			slackClient:      &fakeSlackClient{},
			transcriptParser: &fakeTranscriptParser{},
			llmProvider:      &fakeLLMProvider{},
			githubClient:     &fakeGitHubClient{},
			want:             http.StatusInternalServerError,
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			threadTs := "1234567890"
			messages, _ := json.Marshal(map[string]interface{}{
				"messages": []map[string]interface{}{
					{"user": "U1001", "text": "We need to track this bug", "ts": "1234567890.000001"},
					{"user": "U1002", "text": "Agreed, it keeps happening in prod", "ts": "1234567890.000002"},
					{"user": "U1003", "text": "<@U0001> create an issue from this thread", "ts": "1234567890.123456"},
				},
			})
			transcript, _ := json.Marshal(map[string]interface{}{
				"messages": []map[string]interface{}{
					{"user": "U1001", "text": "We need to track this bug", "ts": "1234567890.000001"},
					{"user": "U1002", "text": "Agreed, it keeps happening in prod", "ts": "1234567890.000002"},
					{"user": "U1003", "text": "<@U0001> create an issue from this thread", "ts": "1234567890.123456"},
				},
			})
			issue, _ := json.Marshal(map[string]interface{}{
				"title":  "Bug report",
				"body":   "Something broke",
				"labels": []string{"bug"},
			})
			issueURL := "https://github.com/testowner/testrepo/issues/42"

			req := httptest.NewRequest("POST", "/pubsub/push", bytes.NewReader(messages))
			rec := httptest.NewRecorder()
			handlePubsubPush(tt.slackClient, tt.transcriptParser, tt.llmProvider, tt.githubClient, threadTs)(rec, req)

			if rec.Code != tt.want {
				t.Errorf("got %d, want %d", rec.Code, tt.want)
			}

			if !bytes.Equal(tt.slackClient.(*fakeSlackClient).fetchThreadGot, []byte(threadTs)) {
				t.Errorf("slack got %q, want %q", tt.slackClient.(*fakeSlackClient).fetchThreadGot, threadTs)
			}

			if !bytes.Equal(tt.transcriptParser.(*fakeTranscriptParser).got, messages) {
				t.Errorf("parser got %q, want %q", tt.transcriptParser.(*fakeTranscriptParser).got, messages)
			}

			if !bytes.Equal(tt.llmProvider.(*fakeLLMProvider).got, transcript) {
				t.Errorf("llm got %q, want %q", tt.llmProvider.(*fakeLLMProvider).got, transcript)
			}

			if !bytes.Equal(tt.githubClient.(*fakeGitHubClient).got, issue) {
				t.Errorf("github got %q, want %q", tt.githubClient.(*fakeGitHubClient).got, issue)
			}

			if !bytes.Equal(tt.slackClient.(*fakeSlackClient).postReplyGot, []byte(issueURL)) {
				t.Errorf("slack got %q, want %q", tt.slackClient.(*fakeSlackClient).postReplyGot, issueURL)
			}
		})
	}
}
