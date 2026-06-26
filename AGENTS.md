# EASEGRESS KNOWLEDGE BASE

## Reply Format (Required)

- Every first reply must begin with: "I have followed the instructions in AGENTS.md."
- Immediately follow with a concise, natural-English refinement of the user's query to aid English learning.

## Overview

Easegress is a Go-based traffic orchestration system. Main binaries:

- `cmd/server`: `easegress-server`
- `cmd/client`: `egctl`
- `cmd/builder`: `egbuilder`

Core runtime flow:

- `pkg/registry` blank-imports built-ins
- filters register through `pkg/filters`
- objects register through `pkg/supervisor`
- traffic gates dispatch to pipelines, which run filter chains

## Key Paths

- `pkg/filters/`: built-in filters
- `pkg/object/`: controllers, traffic gates, pipelines
- `pkg/supervisor/`: object lifecycle and categories
- `pkg/api/`: admin API
- `pkg/option/`: startup flags, env vars, YAML config
- `build/test/`: integration test harness
- `docs/` and `example/`: user-facing docs and sample configs

## Working Rules

- New built-in filters, objects, and routers must both self-register and be added to `pkg/registry/registry.go`, or they will not be included in the server binary.
- Prefer `cmd/client/commandv2/` for CLI work; `cmd/client/command/` is deprecated.
- Use the module path `github.com/megaease/easegress/v2` consistently in source changes.
- Keep the Apache 2.0 license header on new Go files.
- If `--config-file` is used, other CLI flags are ignored.
- Update docs/examples when changing user-visible config, CLI, filter kinds, object kinds, or API behavior.

## Commands

```bash
make build
make fmt
make vet
make test
make integration_test
make wasm
```

Useful variants:

```bash
make test TEST_FLAGS="-race -coverprofile=coverage.txt -covermode=atomic"
EASEGRESS_TEST_SKIP_DOCKER=true make test
go run ./cmd/server --help
go run ./cmd/client --help
go run ./cmd/builder --help
```

## CI Notes

- `test.yml`: unit tests, integration tests, wasm build
- `code.analysis.yml`: `gofmt`, `revive`, misspell
- `golangci.lint.yml`: `golangci-lint`
- `license.yml`: license headers
- `release.yml`: test matrix + GoReleaser on tags

## Pre-PR Checklist

- `make fmt`, `make vet`, and `make test` pass
- Run `make integration_test` for cluster, supervisor, lifecycle, or traffic changes
- Run `make wasm` for `wasmhost` changes
- Keep `go.mod` and `go.sum` tidy unless dependency changes are intentional
- Update docs/examples for user-visible behavior changes
