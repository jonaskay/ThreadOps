package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type fakeSlackClient struct {
	fetchThreadGot string
	fetchThreadErr error
	postReplyGot   string
	postReplyErr   error
}

func (f *fakeSlackClient) FetchThread(ts string) (SlackThread, error) {
	f.fetchThreadGot = ts

	return SlackThread{
		Messages: []SlackMessage{
			{User: "U1001", Text: "We need to track this bug"},
			{User: "U1002", Text: "Agreed, it keeps happening in prod"},
			{User: "U1003", Text: "<@U0001> create an issue from this thread"},
		},
	}, f.fetchThreadErr
}
func (f *fakeSlackClient) PostReply(url string) error {
	f.postReplyGot = url

	return f.postReplyErr
}

type fakeLLMProvider struct {
	got SlackThread
	err error
}

func (f *fakeLLMProvider) Query(transcript SlackThread) (Issue, error) {
	f.got = transcript
	return Issue{
		Title:  "Bug report",
		Body:   "Something broke",
		Labels: []string{"bug"},
	}, f.err
}

type fakeGitHubClient struct {
	got Issue
	err error
}

func (f *fakeGitHubClient) CreateIssue(issue Issue) (string, error) {
	f.got = issue
	return "https://github.com/testowner/testrepo/issues/42", f.err
}

func TestHandlePubsubPush(t *testing.T) {
	test := []struct {
		name         string
		slackClient  SlackClient
		llmProvider  LLMProvider
		githubClient GitHubClient
		wantCode     int
		wantLLM      SlackThread
		wantGitHub   Issue
		wantIssueURL string
	}{
		{
			name:         "valid request",
			slackClient:  &fakeSlackClient{},
			llmProvider:  &fakeLLMProvider{},
			githubClient: &fakeGitHubClient{},
			wantCode:     http.StatusOK,
			wantLLM: SlackThread{
				Messages: []SlackMessage{
					{User: "U1001", Text: "We need to track this bug"},
					{User: "U1002", Text: "Agreed, it keeps happening in prod"},
					{User: "U1003", Text: "<@U0001> create an issue from this thread"},
				},
			},
			wantGitHub: Issue{
				Title:  "Bug report",
				Body:   "Something broke",
				Labels: []string{"bug"},
			},
			wantIssueURL: "https://github.com/testowner/testrepo/issues/42",
		},
		{
			name:         "fetch conversation fails",
			slackClient:  &fakeSlackClient{fetchThreadErr: errors.New("fetch thread failed")},
			llmProvider:  &fakeLLMProvider{},
			githubClient: &fakeGitHubClient{},
			wantCode:     http.StatusInternalServerError,
			wantLLM:      SlackThread{},
			wantGitHub:   Issue{},
			wantIssueURL: "",
		},
		{
			name:         "llm call fails",
			slackClient:  &fakeSlackClient{},
			llmProvider:  &fakeLLMProvider{err: errors.New("llm query failed")},
			githubClient: &fakeGitHubClient{},
			wantCode:     http.StatusInternalServerError,
			wantLLM: SlackThread{
				Messages: []SlackMessage{
					{User: "U1001", Text: "We need to track this bug"},
					{User: "U1002", Text: "Agreed, it keeps happening in prod"},
					{User: "U1003", Text: "<@U0001> create an issue from this thread"},
				},
			},
			wantGitHub:   Issue{},
			wantIssueURL: "",
		},
		{
			name:         "issue creation fails",
			slackClient:  &fakeSlackClient{},
			llmProvider:  &fakeLLMProvider{},
			githubClient: &fakeGitHubClient{err: errors.New("create issue failed")},
			wantCode:     http.StatusInternalServerError,
			wantLLM: SlackThread{
				Messages: []SlackMessage{
					{User: "U1001", Text: "We need to track this bug"},
					{User: "U1002", Text: "Agreed, it keeps happening in prod"},
					{User: "U1003", Text: "<@U0001> create an issue from this thread"},
				},
			},
			wantGitHub: Issue{
				Title:  "Bug report",
				Body:   "Something broke",
				Labels: []string{"bug"},
			},
			wantIssueURL: "",
		},
		{
			name:         "slack reply fails",
			slackClient:  &fakeSlackClient{postReplyErr: errors.New("post reply failed")},
			llmProvider:  &fakeLLMProvider{},
			githubClient: &fakeGitHubClient{},
			wantCode:     http.StatusInternalServerError,
			wantLLM: SlackThread{
				Messages: []SlackMessage{
					{User: "U1001", Text: "We need to track this bug"},
					{User: "U1002", Text: "Agreed, it keeps happening in prod"},
					{User: "U1003", Text: "<@U0001> create an issue from this thread"},
				},
			},
			wantGitHub: Issue{
				Title:  "Bug report",
				Body:   "Something broke",
				Labels: []string{"bug"},
			},
			wantIssueURL: "https://github.com/testowner/testrepo/issues/42",
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			threadTs := "1234567890.123456"
			envelope, _ := json.Marshal(PubSubEnvelope{
				Message: PubSubMessage{
					Data: []byte(`{"type":"event_callback","event":{"type":"app_mention","thread_ts":"` + threadTs + `","ts":"` + threadTs + `"}}`),
				},
			})

			req := httptest.NewRequest("POST", "/pubsub/push", bytes.NewReader(envelope))
			rec := httptest.NewRecorder()
			handlePubsubPush(tt.slackClient, tt.llmProvider, tt.githubClient)(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("got %d, want %d", rec.Code, tt.wantCode)
			}

			if tt.slackClient.(*fakeSlackClient).fetchThreadGot != threadTs {
				t.Errorf("slack got %q, want %q", tt.slackClient.(*fakeSlackClient).fetchThreadGot, threadTs)
			}

			if !reflect.DeepEqual(tt.llmProvider.(*fakeLLMProvider).got, tt.wantLLM) {
				t.Errorf("llm got %v, want %v", tt.llmProvider.(*fakeLLMProvider).got, tt.wantLLM)
			}

			if !reflect.DeepEqual(tt.githubClient.(*fakeGitHubClient).got, tt.wantGitHub) {
				t.Errorf("github got %v, want %v", tt.githubClient.(*fakeGitHubClient).got, tt.wantGitHub)
			}

			if tt.slackClient.(*fakeSlackClient).postReplyGot != tt.wantIssueURL {
				t.Errorf("slack got %q, want %q", tt.slackClient.(*fakeSlackClient).postReplyGot, tt.wantIssueURL)
			}
		})
	}
}

func TestHandlePubsubPushMissingThreadTimestamp(t *testing.T) {
	slackClient := &fakeSlackClient{}
	llmProvider := &fakeLLMProvider{}
	githubClient := &fakeGitHubClient{}

	envelope, _ := json.Marshal(PubSubEnvelope{
		Message: PubSubMessage{
			Data: []byte(`{"type":"event_callback","event":{"type":"app_mention"}}`),
		},
	})

	req := httptest.NewRequest("POST", "/pubsub/push", bytes.NewReader(envelope))
	rec := httptest.NewRecorder()
	handlePubsubPush(slackClient, llmProvider, githubClient)(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	if slackClient.fetchThreadGot != "" {
		t.Fatalf("slack got %q, want empty thread timestamp", slackClient.fetchThreadGot)
	}
}
