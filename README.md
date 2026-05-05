# Warp Local Adapter

A local, open-source, Warp-compatible coding agent adapter for OpenAI-compatible LLM providers.

This project lets a patched Warp client talk to a local Go server instead of Warp's hosted AI backend. The local server accepts Warp's protobuf request format, calls an OpenAI-compatible model endpoint, and streams Warp-compatible SSE/protobuf events back to the client.

## Status

This repository is an MVP focused on the core coding loop:

- read code
- search code
- run shell commands
- wait for long-running shell commands
- apply file diffs
- return streamed assistant output

It is not a full replacement for Warp's production backend.

## What Works

Current MVP tool support:

- `read_files`
- `grep`
- `file_glob`
- `file_glob_v2`
- `run_shell_command`
- `read_shell_command_output`
- `transfer_shell_command_control_to_user`
- `apply_file_diffs`
- `search_codebase`

Current server behavior:

- Warp-compatible `POST /ai/multi-agent`
- protobuf request decoding
- SSE + base64 + protobuf response events
- conversation history persistence
- long-running shell command tracking
- unsupported tool rejection with explicit error text

## What Does Not Yet Work

Not in the current MVP:

- MCP tools
- subagents / multi-agent orchestration
- computer use
- document-specific tools
- passive suggestions
- full Warp server parity

If the model asks for an unsupported tool, the local adapter now rejects it explicitly instead of forwarding a broken tool call to the client.

## Repository Layout

```text
local-adapter/
├── cmd/server/                 # HTTP entrypoint
├── internal/agent/             # system prompt
├── internal/config/            # config loading
├── internal/llm/               # OpenAI-compatible LLM client
├── internal/proto/             # generated Go protobuf files
├── internal/tools/             # local tool implementations
├── proto/                      # protocol source files used by this adapter
├── proto3/                     # compatibility copies used during generation
├── MVP.md                      # implementation notes and architecture history
├── TODO.md                     # working backlog / notes
└── build_and_bundle.sh         # optional macOS WarpLocal bundle helper
```

## Quick Start

### 1. Prerequisites

- Go 1.22 or newer
- A patched Warp client that points to this local adapter
- An OpenAI-compatible endpoint such as:
  - OpenAI
  - OpenRouter
  - Ollama
  - LM Studio
  - vLLM

### 2. Configure

Copy the example config:

```bash
cp config.example.yaml config.yaml
```

Then edit `config.yaml`:

```yaml
provider: openai-compatible
base_url: https://api.openai.com/v1
api_key: YOUR_API_KEY
model: gpt-4.1-mini
server:
  host: 127.0.0.1
  port: 18888
```

### 3. Run the Adapter

```bash
go run ./cmd/server
```

Health check:

```bash
curl http://127.0.0.1:18888/health
```

### 4. Point Warp to the Local Adapter

This repository does not ship an official Warp build. You need a locally patched Warp client that sends AI traffic to:

```text
http://127.0.0.1:18888/ai/multi-agent
```

The helper script `build_and_bundle.sh` shows one way to bundle a local Warp build for macOS. It is an optional local workflow helper, not part of the adapter runtime.

## Configuration

Runtime configuration is loaded from `config.yaml`.

Fields:

- `provider`: free-form label used by the local adapter
- `base_url`: OpenAI-compatible API base URL
- `api_key`: provider API key
- `model`: model name
- `server.host`: bind host
- `server.port`: bind port

Public repositories should not commit real `config.yaml` files. Use `config.example.yaml` as the template.

## Protocol Compatibility and Origins

This project keeps a minimal set of Warp-compatible protocol files so the local adapter can speak the request/response format expected by a patched Warp client.

- `proto/` contains protocol sources used by this adapter workflow
- `internal/proto/` contains generated Go bindings used at runtime

These files are kept here for compatibility with the client protocol used by this project. This repository is not an official Warp backend mirror and does not claim to implement the full hosted service behavior.

## Development

Format and test:

```bash
gofmt -w ./cmd ./internal
go test ./...
```

## Open Source Notes

This repository intentionally excludes:

- local conversation state
- personal config files
- built binaries
- local app bundles

See `.gitignore` for the exact exclusions.

## Roadmap

Near-term priorities:

1. make `apply_file_diffs` failure reporting more structured
2. improve long-running shell command behavior
3. add `ask_user_question` as a late-MVP capability
4. keep reducing accidental reliance on unsupported Warp backend features

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=sasuke39/openWarpAdapter&type=Date)](https://star-history.com/#sasuke39/openWarpAdapter&Date)

## License

MIT. See [LICENSE](./LICENSE).
