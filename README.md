# ThreadOps

ThreadOps is a self-hosted Slack bot that listens for app mentions inside threads, feeds the thread transcript to a configurable LLM, and files the resulting content as a GitHub issue.

## Local development

Prerequisites:

- Go 1.26.1
- Docker and Docker Compose (used to run the Pub/Sub emulator for e2e tests)
- `protoc` (Protocol Buffer compiler) with `protoc-gen-go` plugin
- Terraform CLI
- Google Cloud Platform project with the Pub/Sub API enabled

To sync the dependencies, run

    $ go work sync

### Regenerating proto types

After editing `proto/threadops/v1/slack_event.proto`, regenerate the Go code:

    $ make generate

## Infrastructure (Terraform)

The `terraform/` directory provisions the Pub/Sub schema and topic on Google Cloud.

    $ cd terraform
    $ terraform init
    $ terraform apply -var="project_id=YOUR_GCP_PROJECT_ID"
