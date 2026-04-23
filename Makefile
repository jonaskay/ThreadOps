.PHONY: test test-e2e build fmt vet tidy generate

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
