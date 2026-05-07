# Troubleshooting

## macOS says the app is damaged

The app is currently unsigned. If it was downloaded through a browser, macOS may add a quarantine attribute.

```bash
xattr -cr /Applications/WarpLocal.app
```

## Settings page does not open

Check whether the adapter helper is listening:

```bash
curl http://127.0.0.1:18888/health
```

If nothing is listening, fully quit `WarpLocal.app` and open it again.

## AI requests fail with provider errors

Open:

```text
http://127.0.0.1:18888/settings
```

Confirm:

- base URL includes the expected API root
- API key is present
- model name is supported by the provider
- local providers such as Ollama or LM Studio are already running

## Tool calls repeat

Use the latest release. Older builds had stricter history cleanup that could remove pending tool-call state too early, causing the model to repeat a command after the tool result returned.

## Official Warp quits when WarpLocal opens

Use the latest release. `WarpLocal.app` is designed to coexist with the official Warp app.
