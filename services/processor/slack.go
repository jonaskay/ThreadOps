package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type slackHTTPClient struct {
	baseURL  string
	botToken string
	http     *http.Client
}

func newSlackHTTPClient(baseURL, botToken string) *slackHTTPClient {
	return &slackHTTPClient{
		baseURL:  baseURL,
		botToken: botToken,
		http:     http.DefaultClient,
	}
}

func (c *slackHTTPClient) FetchThread(ts string) (SlackThread, error) {
	u := fmt.Sprintf("%s/api/conversations.replies?ts=%s", c.baseURL, url.QueryEscape(ts))
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return SlackThread{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.botToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return SlackThread{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SlackThread{}, fmt.Errorf("conversations.replies: status %d", resp.StatusCode)
	}

	var thread SlackThread
	if err := json.NewDecoder(resp.Body).Decode(&thread); err != nil {
		return SlackThread{}, fmt.Errorf("decode conversations.replies: %w", err)
	}
	return thread, nil
}

func (c *slackHTTPClient) PostReply(issueURL string) error {
	body, _ := json.Marshal(map[string]string{
		"text": "Issue created: " + issueURL,
	})
	req, err := http.NewRequest("POST", c.baseURL+"/api/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.botToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("chat.postMessage: status %d", resp.StatusCode)
	}
	return nil
}
