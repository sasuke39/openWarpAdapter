---
layout: home

hero:
  name: open-warp
  text: Bring your own AI backend to Warp.
  tagline: A local open-source adapter that routes Warp AI requests to OpenAI-compatible providers such as DeepSeek, OpenAI, Ollama, OpenRouter, LM Studio, and vLLM.
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/sasuke39/open-warp

features:
  - title: Local by design
    details: WarpLocal routes AI requests to a Go helper running on your machine instead of the official Warp cloud backend.
  - title: Bring your own provider
    details: Configure any OpenAI-compatible endpoint, API key, and model from the WarpLocal settings menu.
  - title: Coding-agent workflow
    details: Supports core tools for reading files, searching code, running shell commands, applying file diffs, and collecting command output.
  - title: Works beside official Warp
    details: The patched WarpLocal app uses a separate local channel and can coexist with the official Warp app.
---

## What is open-warp?

Warp's client is open source, but its AI backend is still controlled by the official cloud service. `open-warp` fills that gap with a local adapter server.

The project ships two parts:

- `WarpLocal.app`: a patched Warp app bundle that sends AI traffic to `127.0.0.1:18888`.
- `warp-local-adapter`: a Go backend that translates Warp's protobuf stream into OpenAI-compatible chat completions.

## Current status

`open-warp` is usable for the core coding loop: ask, inspect files, search, run commands, apply diffs, and continue with tool results. It does not yet aim for complete Warp backend parity.

If this project helps you, please star [sasuke39/open-warp](https://github.com/sasuke39/open-warp). That signal helps guide future tool support and release work.
