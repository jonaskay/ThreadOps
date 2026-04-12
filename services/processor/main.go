package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	slackBotToken := os.Getenv("SLACK_BOT_TOKEN")
	if slackBotToken == "" {
		log.Fatal("SLACK_BOT_TOKEN is not set")
	}
	slackAPIURL := os.Getenv("SLACK_API_URL")
	if slackAPIURL == "" {
		slackAPIURL = "https://slack.com"
	}

	anthropicAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicAPIKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is not set")
	}
	anthropicModel := os.Getenv("ANTHROPIC_MODEL")
	if anthropicModel == "" {
		log.Fatal("ANTHROPIC_MODEL is not set")
	}
	anthropicBaseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if anthropicBaseURL == "" {
		anthropicBaseURL = "https://api.anthropic.com"
	}

	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		log.Fatal("GITHUB_TOKEN is not set")
	}
	githubRepo := os.Getenv("GITHUB_REPO")
	if githubRepo == "" {
		log.Fatal("GITHUB_REPO is not set")
	}
	githubBaseURL := os.Getenv("GITHUB_BASE_URL")
	if githubBaseURL == "" {
		githubBaseURL = "https://api.github.com"
	}

	var slackClient SlackClient = newSlackHTTPClient(slackAPIURL, slackBotToken)
	var llmProvider LLMProvider = newAnthropicLLM(anthropicBaseURL, anthropicAPIKey, anthropicModel)
	var githubClient GitHubClient = newGitHubHTTPClient(githubBaseURL, githubToken, githubRepo)

	http.HandleFunc("/pubsub/push", handlePubsubPush(slackClient, llmProvider, githubClient))

	fmt.Printf("processor listening on :%s\n", port)
	http.ListenAndServe(":"+port, nil)
}
