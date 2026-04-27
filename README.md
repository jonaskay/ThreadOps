# ThreadOps

ThreadOps is a self-hosted Slack bot that listens for app mentions inside threads, feeds the thread transcript to a configurable LLM, and files the resulting content as a GitHub issue.

## Local development

Prerequisites:

- Go 1.26.1
- Docker and Docker Compose
- `protoc` (Protocol Buffer compiler) with `protoc-gen-go` plugin
- [ngrok](https://ngrok.com/) to expose the webhook service to Slack
- Terraform CLI (infrastructure only)

To sync the dependencies, run

    $ go work sync

### First-time setup

**1. Create a Slack app**

Go to https://api.slack.com/apps and create a new app from scratch.

Under **OAuth & Permissions**, add these bot token scopes:
- `app_mentions:read`
- `channels:history`
- `groups:history`
- `chat:write`

Install the app to your workspace and copy the **Bot User OAuth Token** (`xoxb-...`).

Copy the **Signing Secret** from **Basic Information → App Credentials**.

Under **Event Subscriptions**, enable events and subscribe to `app_mention` under **Subscribe to bot events**. You'll set the Request URL after starting the service.

**2. Configure credentials**

Copy `.env.example` to `.env` and fill in the values:

    $ cp .env.example .env

**3. Start the Pub/Sub emulator**

    $ make dev-up

Wait a few seconds for the emulator to be ready, then create the topic and push subscription:

    $ make dev-setup

**4. Run the services**

In one terminal (webhook, port 8080):

    $ export $(cat .env | grep -v '^#' | xargs)
    $ go run ./services/webhook

In another terminal (processor, port 8081):

    $ export $(cat .env | grep -v '^#' | xargs)
    $ go run ./services/processor

**5. Expose the webhook with ngrok**

    $ ngrok http 8080

Copy the `https://` forwarding URL.

**6. Register the webhook with Slack**

In your app's **Event Subscriptions** page, set the Request URL to:

    https://<your-ngrok-subdomain>.ngrok-free.app/slack/events

Slack will send a `url_verification` challenge — the webhook service handles this automatically. Save the URL once it shows "Verified".

**7. Test**

Invite the bot to a Slack channel (`/invite @your-app-name`), then mention it inside an existing thread:

    @your-app-name create an issue from this thread

The bot should reply in the thread with a link to the newly created GitHub issue.

**GitHub labels note:** The LLM may suggest label names that don't exist on your repo. If so, the issue will be created without labels on the first run. Pre-create the labels on your repo (e.g. `bug`, `enhancement`) to have them applied automatically.

### Regenerating proto types

After editing `proto/threadops/v1/slack_event.proto`, regenerate the Go code:

    $ make generate

## Infrastructure (Terraform)

The `terraform/` directory provisions the Pub/Sub schema and topic on Google Cloud.

    $ cd terraform
    $ terraform init
    $ terraform apply -var="project_id=YOUR_GCP_PROJECT_ID"
