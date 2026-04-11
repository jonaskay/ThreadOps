package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	var slackClient SlackClient
	var llmProvider LLMProvider
	var githubClient GitHubClient

	http.HandleFunc("/pubsub/push", handlePubsubPush(slackClient, llmProvider, githubClient))

	fmt.Printf("processor listening on :%s\n", port)
	http.ListenAndServe(":"+port, nil)
}
