# Repository Guidelines

## Project Overview

Podium is a high-performance leaderboard service for games and competitive applications. It exposes HTTP and gRPC APIs for scores, ranks, seasons, and player-relative views. Unlike basic Redis sorted-set implementations, Podium preserves arrival order when scores tie, supports standalone Redis and Redis Cluster, and requires no leaderboard provisioning.

## Project Structure & Module Organization

The Go 1.26 codebase has four modules. The root contains `main.go`, CLI commands in `cmd/`, HTTP handlers in `api/`, configuration in `config/`, and workers in `worker/`. Ranking behavior lives in `leaderboard/`; `client/` provides clients, and `proto/` owns API definitions and generated bindings. Test helpers are under `testing/`; benchmarks, deployment files, docs, and container assets live in `bench/`, `deployments/`, `docs/`, and `build/`.

## Build, Test, and Development Commands

- `make setup` downloads dependencies for every Go module.
- `make build` creates `bin/podium`.
- `make run` starts the service with the local configuration.
- `make test-unit` runs Redis-independent tests.
- `make test` runs the server, leaderboard, and client suites and writes coverage profiles.
- `make lint` runs `golangci-lint` across all modules.
- `make proto-check` formats, lints, and builds protobuf schemas; `make proto` regenerates checked-in bindings.
- `make compose-up-dependencies` starts Redis dependencies; finish with `make compose-down`.

Use `make help` for benchmark, Docker, coverage, and integration targets.

## Coding Style & Naming Conventions

Format Go files with `gofmt`; use tabs as emitted by the formatter. Follow idiomatic Go naming: exported identifiers use `PascalCase`, internal identifiers use `camelCase`, and package names are short lowercase words. Keep package responsibilities narrow and match nearby patterns. Never hand-edit generated `*.pb.go`, `*_grpc.pb.go`, or `*.pb.gw.go` files; update the `.proto` source and regenerate them.

## Testing Guidelines

Tests use both standard `testing` (`TestName`) and Ginkgo/Gomega (`Describe`/`It`) in `*_test.go` files. Add focused tests beside changed code and regression coverage for bug fixes. Do not commit focused or disabled Ginkgo specs (`FIt`, `XIt`, `FDescribe`, or `XDescribe`). CI requires at least 80% coverage in every production Go package via `make coverage-check`.

## Commit & Pull Request Guidelines

Recent history follows Conventional Commits, optionally scoped: `feat(leaderboard): ...`, `perf(leaderboard): ...`, `fix: ...`, or `ci: ...`. Use `!` only for breaking changes. Keep commits focused and imperative.

Pull requests should explain the problem and solution, link relevant issues, and list verification commands. Include tests for behavior changes and update API or operational documentation when applicable. Generated protobuf output must be committed and clean after `make proto`.
