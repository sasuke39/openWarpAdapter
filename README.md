# WarpLocal

Run your own LLM inside Warp. A local, open-source adapter that connects a patched Warp terminal to any OpenAI-compatible provider — OpenAI, DeepSeek, Ollama, OpenRouter, and more.

**How it works:** WarpLocal patches the Warp client to route AI requests to a local Go server instead of Warp's cloud backend. The server translates Warp's protobuf protocol into OpenAI-compatible API calls and streams responses back.

## Features

- Works with any OpenAI-compatible endpoint (OpenAI, DeepSeek, Ollama, OpenRouter, vLLM, LM Studio)
- Drop-in WarpLocal.app — double-click to launch, no CLI needed
- Web settings UI at `http://127.0.0.1:18888/settings`
- CJK input support — Chinese/Japanese/Korean text is recognized as AI queries
- Coexists with the official Warp app

## Supported Tools (MVP)

`read_files` · `grep` · `file_glob` · `file_glob_v2` · `run_shell_command` · `read_shell_command_output` · `transfer_shell_command_control_to_user` · `apply_file_diffs` · `search_codebase`

Not yet supported: MCP tools, subagents, computer use, passive suggestions.

## Install

### Option A: Download Release (Recommended)

```bash
sh ./install.sh
```

Downloads the latest `WarpLocal.app` from [GitHub Releases](https://github.com/sasuke39/openWarpAdapter/releases) and installs it.

### Option B: Build from Source

Prerequisites: Go 1.22+, Rust toolchain, [Warp source](https://github.com/nicohman/warp) (v0.2026.04.29)

```bash
# 1. Clone this repo
git clone https://github.com/sasuke39/openWarpAdapter.git
cd openWarpAdapter

# 2. Build the WarpLocal app bundle
WARP_SRC=/path/to/warp-source sh ./build_and_bundle.sh
open ./WarpLocal.app
```

See **[WARP_CLIENT.md](./WARP_CLIENT.md)** for the full patch and build guide.

## Quick Start

1. **Launch** `WarpLocal.app`
2. **Open settings** — the app menu includes a "Local Adapter Settings..." item, or visit [http://127.0.0.1:18888/settings](http://127.0.0.1:18888/settings)
3. **Configure** your provider, API key, and model
4. **Start coding** — press `Cmd+K` in WarpLocal and talk to your LLM

## Configuration

Runtime config is stored in `config.yaml` (or `~/Library/Application Support/WarpLocal/config.yaml` for bundled apps).

```yaml
provider: openai-compatible
base_url: https://api.openai.com/v1
api_key: YOUR_API_KEY
model: gpt-4.1-mini
server:
  host: 127.0.0.1
  port: 18888
```

You can also configure everything through the web settings UI — no manual YAML editing required.

## Repository Layout

```text
├── cmd/server/                 # Go HTTP server (local adapter)
├── internal/agent/             # system prompt
├── internal/config/            # config loading
├── internal/llm/               # OpenAI-compatible LLM client
├── internal/proto/             # generated Go protobuf files
├── internal/tools/             # local tool implementations
├── patches/                    # Warp client patches (5 files)
├── assets/                     # app icon
├── build_and_bundle.sh         # macOS WarpLocal.app builder
├── install.sh                  # one-click installer
├── WARP_CLIENT.md              # full patch + build guide
```

## Warp Client Patches

The `patches/` directory contains 5 patches that modify the Warp client:

| Patch | Purpose |
|-------|---------|
| 0001 | `WarpServerConfig::local_adapter()` — routes requests to `127.0.0.1:18888` |
| 0002 | `Channel::Local` entrypoint — activates local adapter config |
| 0003 | Skip Firebase auth — local adapter doesn't need cloud auth |
| 0004 | CJK natural language detection — Chinese/Japanese/Korean input recognized as AI queries |
| 0005 | "Local Adapter Settings..." menu item in Warp UI |

See **[WARP_CLIENT.md](./WARP_CLIENT.md)** for details on each patch.

## App Bundle Structure

```
WarpLocal.app/
└── Contents/
    ├── MacOS/warp                # WarpLocal main binary
    ├── Helpers/warp-local-adapter # Go AI backend server
    └── Resources/
        ├── config.example.yaml
        └── iconfile.icns
```

The Warp client manages the adapter server lifecycle — it starts the helper automatically and keeps it running.

## Development

```bash
go test ./...
gofmt -w ./cmd ./internal
```

## Roadmap

1. Native Warp settings page for Local Adapter (instead of web UI)
2. `ask_user_question` tool support
3. Better `apply_file_diffs` failure reporting
4. Improved long-running shell command behavior
5. CI-based release automation

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=sasuke39/openWarpAdapter&type=Date)](https://star-history.com/#sasuke39/openWarpAdapter&Date)

## License

MIT. See [LICENSE](./LICENSE).
