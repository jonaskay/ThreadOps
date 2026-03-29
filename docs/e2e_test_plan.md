# E2E Test Implementation Plan

Step-by-step plan for implementing a **failing** end-to-end test (`TestFullPipeline`) as described in GitHub issue #1. The test should compile and run, but **fail** because the application logic doesn't exist yet.

The plan follows a **test-first** approach: write the complete test and its infrastructure first, then scaffold just enough application code to make it compile and run.

---

## Phase 1: Write `TestFullPipeline` and Test Infrastructure

Write the entire e2e test file (`e2e/e2e_test.go`) including the test logic, fake servers, subprocess management, and a Docker Compose file for the Pub/Sub emulator. This phase produces a test that **does not compile yet** — the service binaries and shared types it depends on don't exist. That's intentional: the test defines the contract that the application code must satisfy.

> **Overall approach: Why Docker Compose + subprocesses?**
>
> The Pub/Sub emulator runs in a Docker container managed by Docker Compose.
> The two application services (webhook, processor) run as compiled Go
> subprocesses launched by the test via `exec.Cmd`. Lightweight fakes for
> external APIs (Slack, GitHub, LLM) use Go's `httptest.NewServer`.
>
> This gives us real infrastructure where it matters (Pub/Sub), real binaries
> exercising production code paths (`main.go` wiring, HTTP routing), and fast
> in-process fakes for simple request/response APIs.
>
> Running the test requires two steps:
> ```
> docker compose up -d
> go test ./e2e/... -v -timeout 120s
> ```

- [ ] **1.1 — Create `docker-compose.yml`**
  At repo root, define a service for the Pub/Sub emulator:
  ```yaml
  services:
    pubsub-emulator:
      image: gcr.io/google.com/cloudsdktool/google-cloud-cli:emulators
      command: gcloud beta emulators pubsub start --project=test-project --host-port=0.0.0.0:8085
      ports:
        - "8085:8085"
  ```

  > **Why Docker Compose instead of testcontainers?** Docker Compose keeps the
  > test's Go dependencies minimal — no Docker client libraries, no container
  > runtime code. The emulator lifecycle is managed outside the test process,
  > which is simpler to debug (`docker compose logs`) and doesn't require Ryuk
  > or other cleanup sidecars. The trade-off is that `docker compose up` must
  > run before the test, but this is a single command that's easy to script in
  > CI or a Makefile.
  >
  > **Why the real Pub/Sub emulator instead of a mock?** The Go Pub/Sub SDK
  > (`cloud.google.com/go/pubsub`) automatically redirects to the emulator when
  > `PUBSUB_EMULATOR_HOST` is set — zero code changes needed. A hand-rolled mock
  > would need to reimplement the gRPC Pub/Sub protocol, which is complex and
  > error-prone. The emulator is Google's official tool for exactly this use case.
  > See [Pub/Sub emulator docs](https://cloud.google.com/pubsub/docs/emulator).

- [ ] **1.2 — Create `e2e/go.mod`**
  `module github.com/you/threadops/e2e`. No heavy dependencies needed — the test only uses stdlib (`net/http`, `net/http/httptest`, `os/exec`, `testing`) plus the standard `crypto/hmac` for signature computation.

  > **Why a separate module for tests?** Even without testcontainers, isolating
  > the e2e test in its own module prevents test-only dependencies from leaking
  > into service binaries. It also keeps the `go.work` workspace clean — each
  > module has a clear purpose.

- [ ] **1.2 — `TestFullPipeline` test function**
  Write the top-level test in `e2e/e2e_test.go`. This is the happy-path test that validates the entire data flow: Slack mention in a thread -> webhook -> Pub/Sub -> processor -> GitHub issue + Slack reply.

  The test function should call helpers (written in subsequent steps) to set up infrastructure and then execute the test logic:

  **a) Construct the Slack event payload**
  Build a JSON payload with `type: "event_callback"`, inner event `type: "app_mention"`, non-empty `thread_ts` (e.g. `"1234567890.123456"`), and a `channel` value (e.g. `"C12345"`).

  > **Why `thread_ts` specifically?** A non-empty `thread_ts` indicates the
  > mention happened inside a thread, which is the trigger for ThreadOps to act.
  > The spec says mentions at the top level of a channel (empty `thread_ts`)
  > should be silently ignored.
  > See [Slack `app_mention` event](https://api.slack.com/events/app_mention).

  **b) Compute HMAC signature**
  Using the test signing secret (`test-signing-secret`), compute `v0=<hmac>` from `v0:<timestamp>:<body>`. Set headers `X-Slack-Request-Timestamp` and `X-Slack-Signature`.

  > **Why compute a real signature instead of skipping verification in tests?**
  > Skipping verification would leave a critical security path untested. By
  > computing a valid signature in the test, you verify that the webhook's HMAC
  > check works correctly with real inputs. The signing secret is a known test
  > value, so the computation is deterministic.
  > See [Slack: Verifying requests](https://api.slack.com/authentication/verifying-requests-from-slack).

  **c) POST to webhook**
  `http.Post` to `http://localhost:<webhook-port>/slack/events` with the signed payload. Assert 200 OK response.

  > **Why assert 200 separately?** The webhook must return 200 within 3 seconds
  > per Slack's contract. If this assertion fails, you know the problem is in the
  > webhook (signature, Pub/Sub publish) rather than downstream. It narrows the
  > debugging surface.

  **d) Assert GitHub issue created**
  ```go
  select {
  case req := <-githubIssueCh:
      // assert req.Title != ""
      // assert req.Body != ""
      // assert req.Labels != nil
  case <-time.After(10 * time.Second):
      t.Fatal("timed out waiting for GitHub issue creation")
  }
  ```

  > **Why 10-second timeout?** The message must travel: webhook -> Pub/Sub
  > emulator -> push to processor -> Slack API call -> LLM stub call -> GitHub
  > API call. In CI or under load this chain can take a few seconds. 10 seconds
  > gives ample room while still failing fast compared to the overall 120s test
  > timeout.
  >
  > **Why assert structural properties (non-empty) instead of exact values?**
  > The stub LLM returns known values, so you *could* assert exact strings. But
  > asserting non-emptiness is more resilient to minor format changes and matches
  > the spec's recommendation. If you want exact assertions, check against the
  > stub's hardcoded response from step 1.5.

  **e) Assert Slack reply posted**
  ```go
  select {
  case reply := <-slackReplyCh:
      // assert reply.Text contains the fake issue URL
  case <-time.After(5 * time.Second):
      t.Fatal("timed out waiting for Slack reply")
  }
  ```

  > **Why a shorter timeout (5s) here?** The Slack reply happens immediately
  > after the GitHub issue is created — it's the last step in the pipeline. If
  > the issue was already created (1.2d passed), the reply should arrive almost
  > instantly. A shorter timeout catches regressions faster.
  >
  > **Why assert the issue URL is in the reply text?** This is the key
  > user-visible outcome: the bot posts the GitHub issue link back in the Slack
  > thread. Checking that the fake URL (`https://github.com/testowner/testrepo/issues/42`)
  > appears in the reply text proves the full pipeline connected end to end.

- [ ] **1.3 — Fake GitHub server**
  `httptest.NewServer` handling `POST /repos/{owner}/{repo}/issues`. Validate `Authorization` header. Unmarshal request body. Send it on a `chan GitHubIssueRequest` (buffered, cap 1). Return `{"html_url": "https://github.com/testowner/testrepo/issues/42"}`.

  > **Why `httptest.NewServer` instead of a container or external mock?**
  > `httptest` is Go stdlib, starts in microseconds, and gives you a real HTTP
  > server on localhost. Since we only need one endpoint with a canned response,
  > this is far simpler than running a container like WireMock or Mountebank.
  >
  > **Why a Go channel for assertions?** The GitHub issue creation happens
  > asynchronously (webhook -> Pub/Sub -> processor -> GitHub). A channel lets
  > the test block until the request arrives, with a timeout for failure cases.
  > This is more reliable than `time.Sleep` — the test proceeds instantly when
  > the event arrives rather than waiting a fixed duration.
  > See [httptest.NewServer](https://pkg.go.dev/net/http/httptest#NewServer).

- [ ] **1.4 — Fake Slack API server**
  `httptest.NewServer` handling:
  - `POST /api/conversations.replies` — return canned thread with 2-3 messages, `ok: true`.
  - `POST /api/chat.postMessage` — record posted text on a `chan SlackReply` (buffered, cap 1), return `ok: true`.

  > **Why fake both Slack endpoints?** The processor calls `conversations.replies`
  > to fetch the thread and `chat.postMessage` to post the issue URL back. Both
  > must return valid responses for the pipeline to complete. The canned thread
  > gives you known input to the LLM stub, and the reply channel lets you assert
  > the final output.
  >
  > **Why not use Slack's own test helpers?** Slack doesn't provide an official
  > test server or emulator. Their SDK has some test utilities, but since we
  > dropped `slack-go/slack`, a simple `httptest` server is the lightest option.

- [ ] **1.5 — Stub LLM server**
  `httptest.NewServer` that accepts any POST and returns a hardcoded `IssueContent` JSON: `{"title": "Test Issue", "body": "Test body from thread", "labels": ["bug"]}`.

  > **Why hardcoded JSON instead of validating the prompt?** The e2e test's job
  > is to verify the pipeline wiring (event -> Pub/Sub -> thread fetch -> LLM ->
  > issue -> reply), not to test LLM output quality. A hardcoded response makes
  > assertions deterministic: you know exactly what title, body, and labels to
  > expect in the GitHub issue and Slack reply. Prompt quality is better tested
  > separately with a real LLM in a manual smoke test.

- [ ] **1.6 — Wait for Pub/Sub emulator readiness**
  At the start of the test (or in `TestMain`), poll `http://localhost:8085` until the emulator is responsive. Fail fast with a clear message if it's not reachable (e.g. "Pub/Sub emulator not running — run `docker compose up -d` first").

  > **Why poll instead of assuming it's ready?** Docker Compose starts the
  > container, but the emulator process inside it needs a moment to bind the
  > port. Polling ensures the test doesn't race against emulator startup.
  > A clear error message when the emulator is missing saves debugging time —
  > the most common failure mode is forgetting to run `docker compose up`.

- [ ] **1.7 — Create topic and push subscription on emulator**
  After the emulator is ready, use `net/http` to PUT the topic (`projects/test-project/topics/threadops-events`) and PUT a push subscription pointing to the processor's address at `/pubsub/push`. The processor port must be pre-assigned (find a free port first).

  > **Why raw HTTP PUTs instead of the Go Pub/Sub SDK?** The emulator exposes
  > the same REST API as production Pub/Sub. A couple of `http.NewRequest` calls
  > are simpler than importing `cloud.google.com/go/pubsub` into the e2e module
  > (which would add another heavy dependency). The REST calls are also more
  > explicit and easier to debug.
  >
  > **Why pre-assign the processor port?** The push subscription needs the
  > processor's address at creation time. You can find a free port by
  > binding to `:0`, reading the assigned port, then closing the listener before
  > passing the port to the subprocess. This avoids a chicken-and-egg problem.
  > See [Pub/Sub REST API: create subscription](https://cloud.google.com/pubsub/docs/reference/rest/v1/projects.subscriptions/create).

- [ ] **1.8 — Build and start service subprocesses**

  > **Why run services as subprocesses instead of in-process?**
  >
  > Running the compiled binary via `exec.Cmd` exercises the exact same code path
  > as production: `main.go` parses config, initializes clients, registers routes,
  > and starts the HTTP server. An in-process approach (importing `handler.go` and
  > calling it directly) would skip all of this wiring and miss bugs like missing
  > env var handling, incorrect route registration, or port binding issues.

  **a) Build service binaries**
  In test setup, `go build -o <tmpdir>/processor ./services/processor` and `go build -o <tmpdir>/webhook ./services/webhook`.

  > **Why build in the test instead of assuming pre-built binaries?** Building
  > fresh ensures the test always runs against the current code. Using `t.TempDir()`
  > for output keeps the working tree clean and automatically cleans up.

  **b) Start processor subprocess**
  Launch the processor binary via `exec.Cmd` with env vars:
  - `PUBSUB_EMULATOR_HOST` → emulator host:port
  - `SLACK_BOT_TOKEN` → `test-slack-token`
  - `SLACK_API_URL` → fake Slack server URL
  - `GITHUB_TOKEN` → `test-github-token`
  - `GITHUB_REPO` → `testowner/testrepo`
  - `GITHUB_BASE_URL` → fake GitHub server URL
  - `LLM_PROVIDER` → `stub` (or any value)
  - `LLM_API_KEY` → `test-key`
  - `STUB_LLM_URL` → stub LLM server URL
  - `LLM_MODEL` → `test-model`
  - `PORT` → pre-assigned free port

  Poll `http://localhost:<port>/` until ready.

  > **Why start the processor before the webhook?** The Pub/Sub push
  > subscription (created in 1.7) points at the processor. If the processor
  > isn't up when the webhook publishes a message, Pub/Sub will attempt delivery,
  > fail, and retry — which still works but adds latency. Starting the processor
  > first ensures it's ready to receive on the first attempt.
  >
  > **Why poll for readiness?** Subprocess startup is async. The binary needs
  > time to load config, connect to the emulator, and bind the port. Polling
  > (e.g. `GET /` or a TCP dial on the port) is more reliable than a fixed
  > sleep.

  **c) Start webhook subprocess**
  Launch the webhook binary via `exec.Cmd` with env vars:
  - `PUBSUB_EMULATOR_HOST` → emulator host:port
  - `SLACK_SIGNING_SECRET` → `test-signing-secret`
  - `SLACK_BOT_TOKEN` → `test-slack-token`
  - `PUBSUB_TOPIC` → `projects/test-project/topics/threadops-events`
  - `PORT` → another pre-assigned free port

  Poll until ready.

  > **Why does the webhook need `SLACK_SIGNING_SECRET` but not `GITHUB_TOKEN`?**
  > The webhook only verifies signatures and publishes to Pub/Sub — it never
  > touches GitHub or the LLM. Each service gets only the secrets it needs, which
  > matches the production IAM setup where the webhook service account can't
  > access GitHub or LLM secrets.

- [ ] **1.9 — Teardown helper**
  `t.Cleanup` that kills both subprocesses and stops all httptest servers. The Pub/Sub emulator container is managed by Docker Compose and cleaned up separately (`docker compose down`).

  > **Why `t.Cleanup` instead of `defer`?** `t.Cleanup` functions run even if
  > the test calls `t.Fatal`, which `defer` in the test function body would also
  > do — but `t.Cleanup` is idiomatic for test teardown and can be registered
  > close to the setup code rather than at the top of the function. It also works
  > correctly with subtests.
  > See [testing.T.Cleanup](https://pkg.go.dev/testing#T.Cleanup).

---

## Phase 2: Application Code Scaffolding

Minimal scaffolding so that both services **compile and start an HTTP server**, but contain **no real application logic**. Handlers should return placeholder responses (e.g. 200 OK with no processing). The e2e test exercises real compiled binaries, so the code must build — but the pipeline should not work end-to-end.

- [ ] **2.1 — Scaffold `internal/` module**
  Create `internal/go.mod` (`module github.com/you/threadops/internal`) and the package directories: `config/`, `slack/`, `github/`, `llm/`.

  > **Why a separate module?** The spec uses a multi-module Go workspace so each
  > service's Docker image only pulls the dependencies it needs. The `internal/`
  > module holds shared code consumed by both services via `replace` directives.
  > See [Go workspaces](https://go.dev/doc/tutorial/workspaces) and
  > [multi-module repos](https://go.dev/doc/modules/managing-source#multiple-module-source).

- [ ] **2.2 — Stub `internal/config`**
  `config.go`: `Config` struct with fields for all env vars (`SLACK_BOT_TOKEN`, `SLACK_SIGNING_SECRET`, `GITHUB_TOKEN`, `GITHUB_REPO`, `LLM_PROVIDER`, `LLM_MODEL`, `LLM_API_KEY`, `PUBSUB_TOPIC`, `PORT`). `Load()` function that reads env vars — no validation logic needed yet.

  > **Why include all fields now?** The services need to accept these env vars
  > to start up in the e2e test. The struct and `Load()` must exist so
  > `main.go` compiles, but validation (e.g. collecting missing vars into one
  > error) is application logic that belongs in a later issue.

- [ ] **2.3 — Stub `internal/slack/types.go`**
  Define `EventCallback`, `Event`, and `Message` structs with JSON tags.

  > **Why hand-rolled structs instead of `slack-go/slack`?**
  > The spec drops the `slack-go/slack` library. ThreadOps only uses a tiny
  > subset of Slack's API surface (event verification, `conversations.replies`,
  > `chat.postMessage`), so defining three small structs avoids pulling in a
  > large dependency with its own transitive tree. See
  > [Slack Events API payload reference](https://api.slack.com/events/app_mention).

- [ ] **2.4 — Stub `internal/slack/verifier.go`**
  `Verify(signingSecret string, r *http.Request, body []byte) error` — return `nil` (no-op). Real HMAC-SHA256 verification is application logic for a later issue.

  > **Why stub instead of implement?** The goal of this issue is a failing e2e
  > test that proves the test infrastructure works. Signature verification is
  > application logic. Stubbing it as a no-op lets the webhook accept requests
  > so the test can progress far enough to fail at the right place (pipeline
  > completion), not at signature checking.

- [ ] **2.5 — Stub `internal/slack/client.go`**
  `Client` struct with `NewClient(token, baseURL string)`, `FetchThread(ctx, channel, threadTS) ([]Message, error)`, and `PostReply(ctx, channel, threadTS, text) error`. All methods return zero values / nil. Must accept a configurable base URL so the e2e test can redirect calls to the fake Slack server.

  > **Why a configurable base URL even in stubs?** The e2e test passes
  > `SLACK_API_URL` as an env var to the subprocess. The constructor must accept
  > it now so the wiring compiles, even though the methods are no-ops.

- [ ] **2.6 — Stub `internal/github/client.go`**
  `Client` struct with `NewClient(token, repo, baseURL string)` and `CreateIssue(ctx, title, body string, labels []string) (string, error)`. Return zero values / nil. Must accept a configurable `baseURL` parameter.

  > **Why not `google/go-github`?** Same rationale as the Slack client — we only
  > call one endpoint (`POST /repos/{owner}/{repo}/issues`), so a 5-line HTTP
  > call (when implemented) is simpler than a large SDK. The configurable base
  > URL is the same testing seam pattern used by `go-github` itself internally.

- [ ] **2.7 — Stub `internal/llm/provider.go`**
  `Provider` interface with `Complete(ctx context.Context, systemPrompt, userPrompt string) (IssueContent, error)`. `IssueContent` struct (`Title`, `Body`, `Labels`). `NewProvider(cfg)` factory that returns a no-op implementation. Accept `STUB_LLM_URL` env var for later use.

  > **Why `STUB_LLM_URL` instead of an interface mock?** The processor runs as a
  > compiled subprocess in the e2e test, so you can't inject a Go interface at
  > runtime. An env-var-controlled URL redirect is the only seam that works
  > across process boundaries without restructuring the code.

- [ ] **2.8 — Scaffold `services/webhook/` module**
  `go.mod`, `main.go` (config load, HTTP server on `PORT`), `handler.go` (POST `/slack/events` — return 200 with empty body, no real logic).

  > **Why no real logic?** The issue requires the test to fail because
  > application logic doesn't exist yet. The webhook must start and bind a port
  > (so the test can POST to it), but it should not verify signatures, parse
  > events, or publish to Pub/Sub.

- [ ] **2.9 — Scaffold `services/processor/` module**
  `go.mod`, `main.go` (config load, HTTP server on `PORT`), `handler.go` (POST `/pubsub/push` — return 200 with empty body, no real logic).

  > **Why no real logic?** Same rationale. The processor must start and bind a
  > port, but it should not decode Pub/Sub messages, fetch threads, call the
  > LLM, or create GitHub issues. This is what makes the e2e test fail.

- [ ] **2.10 — Create `go.work` workspace file**
  At repo root, referencing `./internal`, `./services/webhook`, `./services/processor`, `./e2e`.

  > **Why `go.work`?** Without it, each module's `replace` directive only works
  > inside Docker builds. The workspace file lets `go build`, `go test`, and
  > editor tooling (gopls) resolve cross-module references during local
  > development without publishing anything. It is not used inside containers.
  > See [Go workspace reference](https://go.dev/ref/mod#workspaces).

- [ ] **2.11 — Verify both services compile**
  `go build ./services/webhook` and `go build ./services/processor` must succeed.

  > **Why gate on this?** Catching compilation errors here is much faster than
  > debugging them during the e2e test, where the failure would surface as a
  > cryptic subprocess exit.

---

## Phase 3: Run and Validate

- [ ] **3.1 — Run the test**
  ```
  docker compose up -d
  go test ./e2e/... -v -timeout 120s
  docker compose down
  ```

  > **Why `-timeout 120s`?** The default Go test timeout is 10 minutes, which is
  > far too long to wait if something hangs. 120 seconds is generous for the
  > test case, but short enough to fail promptly. The `-v` flag shows per-test
  > output so you can see which step timed out.

- [ ] **3.2 — Verify the test fails with a clear assertion error**
  The test should fail with a message like "timed out waiting for GitHub issue creation". This confirms the test infrastructure works correctly (Pub/Sub emulator, service subprocesses, fake servers) but the pipeline doesn't complete because the application logic is stubbed out.

  > **Why must the test fail?** This issue is about building the test harness,
  > not the application. A failing test with a clear error message proves the
  > infrastructure is wired correctly and gives us a concrete target for the
  > next issue: implement the application logic until this test passes.
