# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Go monorepo of event-driven microservices built on `github.com/distributed-programming-2026/go-sdk`. A Go workspace (`go.work`) contains a shared `lib` module plus one Go module per service under `modules/<name>` (currently `order`, `delivery`, `payment`). All tooling runs through **mise** (a monorepo with `config_roots = lib, modules/*`). Generated code (`*.pb.go`, `*.gen.go`) is git-ignored, so run `generate` before building after a fresh checkout. The READMEs are in Russian.

`order`, `delivery` and `payment` were just generated from the template. They still contain only the template's `echo` example (domain, migration `version1791225168`, `repository/echo.go`). Real business logic replaces that example.

## Commands

Run these from the repo root. Module tasks are addressed as `//modules/<name>:<task>` or `//lib:<task>`.

```sh
mise trust && mise install                    # toolchain: go 1.27.1, golangci-lint, protoc, oapi-codegen, options-gen
mise run all                                  # every lib+module: generate, tidy, build, test, integration-tests, lint (sequential)
mise run //modules/order:all
mise run //modules/order:generate             # protoc (+ //lib:generate) + oapi-codegen + go generate
mise run //modules/order:build                # linux/CGO=0 binary → modules/order/dist/order (GOARCH overridable)
mise run //modules/order:test                 # unit tests; excludes internal/integration-tests
mise run //modules/order:integration-tests    # builds binary + docker image, then testcontainers (needs Docker)
mise run //modules/order:lint                 # golangci-lint run
mise run //modules/order:generate-migration "Create invoices table"
mise run generate-module billing              # new module from the template, added to go.work
mise run check-generator                      # end-to-end generator check in a temp copy (slow)
mise run check-local-env                      # Python unit tests for gateway config collection
```

Run a single test from inside the module directory, after generating code:

```sh
cd modules/order && go test ./internal/order/domain -run TestName
cd modules/order && go test -v -count=1 ./internal/integration-tests -run TestName   # needs image distributed-programming-order:integration (built by the integration-tests task)
```

`mise-env/docker.sh` is sourced automatically. It exports `DOCKER_HOST` from the active docker context and sets the Ryuk socket override for Colima, Lima and Docker Desktop, so testcontainers works on macOS.

### Local environment

```sh
mise run local-up       # build all modules, validate gateway config, start gateway+MySQL+RabbitMQ+services
mise run local-config | local-status | local-logs | local-down
```

`mise-tasks/local-env` (Python) composes the root `compose.yaml` with every `modules/*/compose.yaml`. It runs each module's `docker/mysql/init.sql` on every `up` and assigns host ports sequentially from 8081 (gRPC, HTTP, consumer health per module). You can override them with `<MODULE>_GRPC_PORT`, `<MODULE>_HTTP_PORT` and `<MODULE>_MESSAGE_HANDLER_PORT`. The gateway listens on `:80` (health on `:15000`) and routes `/<name>/api/...` to `<name>-service:8082`, stripping the `/<name>` prefix.

Gateway config is flattened (not merged) from `config/gateway/` and `modules/*/config/gateway/` into `dist/local/gateway/`. Never edit that directory by hand. The gateway image merges the YAML by `kind`/`weight` and has no hot reload, so re-run `local-up` after changing the config.

## Architecture

### Module template is the source of truth

`mise-tasks/generate-module.d/**.tpl` is the module skeleton. It is not part of `go.work` or the mise config roots. `generate-module` copies it and strips `.tpl`. It also replaces `template` with `<name>`, `Template` with `<TitleName>` and `TEMPLATE` with `<NAME>` (paths, the Go module path, proto packages, env prefix), renames `internal/template`, runs codegen, and runs `go work use`. Structural changes that every module needs belong in the template, and `check-generator` validates them. Existing modules do not pick up template changes automatically.

### Per-module layout (hexagonal)

- `cmd/`: one binary with two `urfave/cli` commands. Each `dependencies_*.go` file wires one subsystem (MySQL, migrations, AMQP, events, servers).
  - `service` (default): HTTP (8082) + gRPC (8081, including the gateway interceptor) + the outbox relay.
  - `message-handler`: the AMQP consumer plus `/healthz`.

  Both commands apply all migrations: module, outbox, inbox.
- `internal/<name>/domain`: models, domain services, repository interfaces, event payloads.
- `internal/<name>/app`: the transaction boundary. It uses `uow.UnitOfWorkWithRepositoryProvider[*mysql.RepositoryProvider]`, and inside `ExecuteWithRepositoryProvider` it builds domain services from `provider.X(ctx)` and `dispatcherFactory.NewDispatcher(ctx)`. Domain writes and outbox events commit in one MySQL transaction.
- `internal/<name>/infra/mysql`: the repository provider, `repository/` implementations, and `migrations/`. The migration registry lives in `factory.go`; always add migrations through `generate-migration`. A new migration's `Up` returns an error until you implement it.
- `internal/<name>/infra/handlers/{public,internalapi,events,gateway}`: the OpenAPI strict server, the gRPC internal API, the AMQP event consumer (inbox-deduplicated), and the gateway gRPC interceptor example.
- `internal/<name>/config`: an envconfig struct with the env prefix `<NAME>_`. The README of each module has a table of the variables.
- `api/`: contracts. `events/events.proto`, `server/internalapi/*.proto` and `server/publicapi/*.oapi.yaml` generate Go code next to the schemas.

### Messaging

- Every module publishes to the shared topic exchange `domain-event`, defined in `lib/event`, with routing key `<name>.<EventType>`. Dead letters go to `domain-event.dead` → `<queue>.dead`.
- Each module declares its own queues and bindings.
- Delivery is at-least-once via the outbox (relay with publisher confirms) and the inbox `(consumer, event_id)` dedup. Consumers have no automatic retry.

### Protobuf conventions

- `protoc` runs with `--proto_path=modules/`, so descriptor paths include the module name (`order/api/events/events.proto`). This prevents registry collisions when one binary imports several modules' contracts.
- Proto imports are written relative to `modules/`, e.g. `import "order/internal/messages/types.proto";`.
- Cross-directory imports need a full `go_package`, e.g. `order/internal/messages;messages`.
- To use another module's contracts, add `require <mod> v0.0.0` plus `replace <mod> => ../<mod>` to `go.mod`.

### lib

`lib` (module `github.com/distributed-programming-2026/lib`) holds:
- the shared event exchange and dispatcher (`lib/event`);
- runtime helpers for signals and HTTP/gRPC servers (`lib/runtime`);
- a vendored copy of the gateway interceptor contract (`lib/gateway/interceptor/interceptorgrpc.proto`, gateway `v0.0.1`; only `go_package` was changed). Keep it in sync with the gateway when you upgrade.

### Docker

Each module's `Dockerfile` only copies the prebuilt `dist/<name>` binary, so run `build` before `docker build`. Integration tests start MySQL, RabbitMQ, the service, and the consumer from the image `distributed-programming-<name>:integration` in an isolated testcontainers network.
