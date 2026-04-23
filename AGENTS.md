# Project overview

ThreadOps is a Slack-to-GitHub automation service written in Go.

## Repository layout

```
go.work                      ← workspace file (local dev only)
internal/                    ← shared library module
services/
  webhook/                   ← webhook service module
  processor/                 ← processor service module
e2e/                         ← e2e test module (requires Docker)
docs/
  app_spec.txt               ← XML spec; source of truth for architecture
```

## Validation

### Unit tests

```sh
make test
```

### E2E tests

```sh
make test-e2e
```

### Build check

```sh
make build
```

### Formatting and vet

```sh
make fmt
make vet
```

### Module tidiness

```sh
make tidy
```
