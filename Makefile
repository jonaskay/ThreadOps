.PHONY: test test-e2e build fmt vet tidy generate dev-up dev-setup dev-down

test:
	go test ./internal/... ./services/webhook/... ./services/processor/...

test-e2e:
	docker compose -f e2e/docker-compose.yml up -d
	go test ./e2e/... ; docker compose -f e2e/docker-compose.yml down -v

build:
	go build ./services/processor ./services/webhook

fmt:
	@unformatted=$$(git ls-files '*.go' | xargs gofmt -l); \
	if [ -n "$$unformatted" ]; then echo "Unformatted files:\n$$unformatted"; exit 1; fi

vet:
	go vet ./internal/... ./services/webhook/... ./services/processor/... ./e2e/...

tidy:
	go work sync && for dir in internal services/webhook services/processor e2e; do (cd "$$dir" && go mod tidy); done

generate:
	protoc --go_out=. --go_opt=module=github.com/jonaskay/threadops proto/threadops/v1/slack_event.proto

dev-up:
	docker compose up -d

dev-setup:
	@echo "Creating Pub/Sub topic and push subscription in emulator..."
	curl -sf -X PUT "http://localhost:8085/v1/projects/threadops-dev/topics/slack-events" > /dev/null
	curl -sf -X PUT "http://localhost:8085/v1/projects/threadops-dev/subscriptions/slack-events-sub" \
		-H "Content-Type: application/json" \
		-d '{"topic":"projects/threadops-dev/topics/slack-events","pushConfig":{"pushEndpoint":"http://host.docker.internal:8081/pubsub/push"}}' > /dev/null
	@echo "Done."

dev-down:
	docker compose down -v
