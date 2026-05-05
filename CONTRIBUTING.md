# Contributing

Thanks for taking a look at this project.

## Development Setup

1. Install Go 1.22 or newer.
2. Copy `config.example.yaml` to `config.yaml`.
3. Fill in your local model endpoint settings.
4. Run:

```bash
go run ./cmd/server
```

## Before Sending Changes

Please run:

```bash
gofmt -w ./cmd ./internal
go test ./...
```

## Scope

This repository is intentionally scoped to a local Warp-compatible coding adapter MVP.

Good contributions:

- improve the core coding loop
- improve protocol compatibility for already-scoped MVP tools
- improve developer docs
- improve failure reporting and debuggability

Please avoid expanding the project into full Warp backend parity in one large change. Smaller, reviewable improvements are much easier to land.

## Sensitive Files

Do not commit:

- `config.yaml`
- `conversations.json`
- local binaries
- app bundles

Use `config.example.yaml` for config-related changes.
