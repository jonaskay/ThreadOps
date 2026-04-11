package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	pubsubEmulatorHost  = "localhost:8085"
	pubsubProject       = "test-project"
	pubsubTopic         = "threadops-events"
	testAnthropicAPIKey = "test-anthropic-api-key"
	testSlackToken      = "test-slack-token"
	testGitHubToken     = "test-github-token"
	testGitHubRepo      = "testowner/testrepo"
)

type env struct {
	WebhookPort   int
	ProcessorPort int
	LLMCallCh     chan AnthropicMessageRequest
	GitHubIssueCh chan GitHubIssueRequest
	SlackThreadCh chan string
	SlackReplyCh  chan SlackReply
	LLMServer     *httptest.Server
	GitHubServer  *httptest.Server
	SlackServer   *httptest.Server
}

func newEnv(t *testing.T) *env {
	t.Helper()

	waitForPubSubEmulator(t)

	processorPort := freePort(t)
	webhookPort := freePort(t)

	llmCallCh := make(chan AnthropicMessageRequest, 1)
	llmServer := FakeAnthropicServer(t, llmCallCh)
	t.Cleanup(llmServer.Close)

	githubIssueCh := make(chan GitHubIssueRequest, 1)
	githubServer := FakeGitHubServer(t, githubIssueCh)
	t.Cleanup(githubServer.Close)

	slackThreadCh := make(chan string, 1)
	slackReplyCh := make(chan SlackReply, 1)
	slackServer := FakeSlackServer(t, slackThreadCh, slackReplyCh)
	t.Cleanup(slackServer.Close)

	createPubSubTopicAndSubscription(t, processorPort)

	binDir := t.TempDir()
	repoRoot := repoRootDir(t)

	buildService(t, repoRoot, binDir, "processor")
	buildService(t, repoRoot, binDir, "webhook")

	startProcessor(t, binDir, processorPort, slackServer.URL, githubServer.URL, llmServer.URL)
	startWebhook(t, binDir, webhookPort)

	return &env{
		WebhookPort:   webhookPort,
		ProcessorPort: processorPort,
		LLMCallCh:     llmCallCh,
		GitHubIssueCh: githubIssueCh,
		SlackReplyCh:  slackReplyCh,
		LLMServer:     llmServer,
		GitHubServer:  githubServer,
		SlackServer:   slackServer,
	}
}

func waitForPubSubEmulator(t *testing.T) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", pubsubEmulatorHost, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatal("Pub/Sub emulator not running — run `docker compose up -d` first")
}

func createPubSubTopicAndSubscription(t *testing.T, processorPort int) {
	t.Helper()

	emulatorURL := "http://" + pubsubEmulatorHost

	// Create topic.
	topicURL := fmt.Sprintf("%s/v1/projects/%s/topics/%s", emulatorURL, pubsubProject, pubsubTopic)
	req, _ := http.NewRequest("PUT", topicURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		t.Fatalf("create topic: status %d", resp.StatusCode)
	}

	// Delete any pre-existing subscription so the push endpoint reflects this run's processor port.
	// The emulator persists subscriptions across test runs, and freePort returns a different port each time.
	subURL := fmt.Sprintf("%s/v1/projects/%s/subscriptions/threadops-push", emulatorURL, pubsubProject)
	delReq, _ := http.NewRequest("DELETE", subURL, nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("delete subscription: %v", err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK && delResp.StatusCode != http.StatusNotFound {
		t.Fatalf("delete subscription: status %d", delResp.StatusCode)
	}

	// Create push subscription.
	subBody, _ := json.Marshal(map[string]interface{}{
		"topic": fmt.Sprintf("projects/%s/topics/%s", pubsubProject, pubsubTopic),
		"pushConfig": map[string]string{
			"pushEndpoint": fmt.Sprintf("http://host.docker.internal:%d/pubsub/push", processorPort),
		},
	})
	req, _ = http.NewRequest("PUT", subURL, bytes.NewReader(subBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		t.Fatalf("create subscription: status %d", resp.StatusCode)
	}
}

func repoRootDir(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Dir(wd)
}

func freePort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func buildService(t *testing.T, repoRoot, binDir, name string) {
	t.Helper()

	cmd := exec.Command("go", "build", "-o", filepath.Join(binDir, name), fmt.Sprintf("./services/%s", name))
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, out)
	}
}

func startProcessor(t *testing.T, binDir string, port int, slackURL, githubURL, llmURL string) {
	t.Helper()

	cmd := exec.Command(filepath.Join(binDir, "processor"))
	cmd.Env = []string{
		fmt.Sprintf("PUBSUB_EMULATOR_HOST=%s", pubsubEmulatorHost),
		fmt.Sprintf("SLACK_BOT_TOKEN=%s", testSlackToken),
		fmt.Sprintf("SLACK_API_URL=%s", slackURL),
		fmt.Sprintf("GITHUB_TOKEN=%s", testGitHubToken),
		fmt.Sprintf("GITHUB_REPO=%s", testGitHubRepo),
		fmt.Sprintf("GITHUB_BASE_URL=%s", githubURL),
		"LLM_PROVIDER=stub",
		"LLM_API_KEY=test-key",
		fmt.Sprintf("STUB_LLM_URL=%s", llmURL),
		"LLM_MODEL=test-model",
		fmt.Sprintf("PORT=%d", port),
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start processor: %v", err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	waitForService(t, port, "processor")
}

func startWebhook(t *testing.T, binDir string, port int) {
	t.Helper()

	cmd := exec.Command(filepath.Join(binDir, "webhook"))
	cmd.Env = []string{
		fmt.Sprintf("PUBSUB_EMULATOR_HOST=%s", pubsubEmulatorHost),
		fmt.Sprintf("PROJECT_ID=%s", pubsubProject),
		fmt.Sprintf("SLACK_SIGNING_SECRET=%s", testSigningSecret),
		fmt.Sprintf("SLACK_BOT_TOKEN=%s", testSlackToken),
		fmt.Sprintf("PUBSUB_TOPIC=projects/%s/topics/%s", pubsubProject, pubsubTopic),
		fmt.Sprintf("PORT=%d", port),
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start webhook: %v", err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	waitForService(t, port, "webhook")
}

func waitForService(t *testing.T, port int, name string) {
	t.Helper()

	url := fmt.Sprintf("http://localhost:%d/", port)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("%s did not become ready on port %d", name, port)
}
