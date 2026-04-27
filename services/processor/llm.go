package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type anthropicLLM struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func newAnthropicLLM(baseURL, apiKey, model string) *anthropicLLM {
	return &anthropicLLM{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		http:    http.DefaultClient,
	}
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (c *anthropicLLM) Query(transcript SlackThread) (Issue, error) {
	var b strings.Builder
	for _, m := range transcript.Messages {
		b.WriteString(m.User)
		b.WriteString(": ")
		b.WriteString(m.Text)
		b.WriteString("\n")
	}

	prompt := "Convert the following Slack thread into a GitHub issue. " +
		"Respond with ONLY a single JSON object — no explanation, no markdown — containing \"title\", \"body\", and \"labels\" fields.\n\n" +
		"Thread:\n" + b.String()

	reqBody, _ := json.Marshal(anthropicRequest{
		Model:     c.model,
		MaxTokens: 1024,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	})

	req, err := http.NewRequest("POST", c.baseURL+"/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		return Issue{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Anthropic-Version", "2023-06-01")

	resp, err := c.http.Do(req)
	if err != nil {
		return Issue{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Issue{}, fmt.Errorf("anthropic /v1/messages: status %d: %s", resp.StatusCode, body)
	}

	var r anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Issue{}, fmt.Errorf("decode anthropic response: %w", err)
	}

	if len(r.Content) == 0 {
		return Issue{}, fmt.Errorf("anthropic response has no content")
	}

	text := extractJSON(r.Content[0].Text)
	var issue Issue
	if err := json.Unmarshal([]byte(text), &issue); err != nil {
		return Issue{}, fmt.Errorf("parse issue JSON from llm: %w", err)
	}

	return issue, nil
}

// extractJSON finds the first JSON object in s, stripping any surrounding prose or code fences.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	for _, fence := range []string{"```json", "```"} {
		if strings.HasPrefix(s, fence) {
			s = strings.TrimPrefix(s, fence)
			s = strings.TrimSuffix(s, "```")
			return strings.TrimSpace(s)
		}
	}
	// Fallback: extract the first {...} block in case the LLM added surrounding prose.
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end > start {
		return s[start : end+1]
	}
	return s
}
