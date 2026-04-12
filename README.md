# ThreadOps

ThreadOps is a self-hosted Slack bot that listens for app mentions inside threads, feeds the thread transcript to a configurable LLM, and files the resulting content as a GitHub issue.

## Local development

Prerequisites:

- Go 1.26.1
- Docker and Docker Compose (used to run the Pub/Sub emulator for e2e tests)

To sync the dependencies, run

    $ go work sync