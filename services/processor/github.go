package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type githubHTTPClient struct {
	baseURL string
	token   string
	repo    string
	http    *http.Client
}

func newGitHubHTTPClient(baseURL, token, repo string) *githubHTTPClient {
	return &githubHTTPClient{
		baseURL: baseURL,
		token:   token,
		repo:    repo,
		http:    http.DefaultClient,
	}
}

func (c *githubHTTPClient) CreateIssue(issue Issue) (string, error) {
	htmlURL, status, err := c.createIssue(issue)
	if err != nil {
		return "", err
	}
	// Labels must pre-exist on the repo; retry without them on validation error.
	if status == http.StatusUnprocessableEntity && len(issue.Labels) > 0 {
		issue.Labels = nil
		htmlURL, _, err = c.createIssue(issue)
		if err != nil {
			return "", err
		}
	}
	return htmlURL, nil
}

func (c *githubHTTPClient) createIssue(issue Issue) (string, int, error) {
	body, _ := json.Marshal(issue)
	url := fmt.Sprintf("%s/repos/%s/issues", c.baseURL, c.repo)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "token "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", resp.StatusCode, fmt.Errorf("github create issue: status %d: %s", resp.StatusCode, respBody)
	}

	var r struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", resp.StatusCode, fmt.Errorf("decode github response: %w", err)
	}
	return r.HTMLURL, resp.StatusCode, nil
}
