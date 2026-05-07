# Getting Started

`open-warp` lets you use your own OpenAI-compatible LLM provider inside Warp through the `WarpLocal.app` bundle.

## Install from release

Download the latest `WarpLocal.app.zip` from:

https://github.com/sasuke39/open-warp/releases

Unzip it, move `WarpLocal.app` to `/Applications`, then open it.

If macOS says the app is damaged, clear the quarantine attribute:

```bash
xattr -cr /Applications/WarpLocal.app
```

The install script also handles this automatically:

```bash
sh ./install.sh
```

## Configure your provider

Open `WarpLocal.app`, then choose `Local Adapter Settings...` from the app menu.

Fill in the fields shown in the settings window:

- provider name
- base URL
- API key
- model name

Then click save. The adapter reloads the configuration without a full app restart.

The local HTTP settings URL is still available for debugging, but normal users do not need to open it manually.

## Start using AI in WarpLocal

Open `WarpLocal.app`, press `Cmd+K`, and ask a natural-language question.

Examples:

```text
Explain the current directory.
Find the server entry point.
Create a simple Go HTTP handler and run the tests.
```

Chinese, Japanese, and Korean input is detected as natural language and should not fall through to the shell.
