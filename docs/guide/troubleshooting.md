# Troubleshooting

## Generate diagnostics for a bug report

If WarpLocal crashes or behaves unexpectedly, run the diagnostics command and attach the output to your bug report.

**If you have the repo:**

```bash
sh install.sh doctor
```

**If you only have the app (no repo):**

```bash
# Option 1: Run from the app bundle
bash /Applications/WarpLocal.app/Contents/Resources/diagnostics.sh

# Option 2: One-line download and run
bash <(curl -fsSL https://raw.githubusercontent.com/sasuke39/open-warp/main/diagnostics.sh)
```

This generates a folder on your Desktop containing:

| File | Contents |
|------|----------|
| `diagnostics.json` | Machine-readable system, app, and config metadata |
| `issue-summary.md` | Copy-paste summary for your GitHub issue |
| `warplocal-log-tail.txt` | Last 300 lines of the adapter log (redacted) |
| `crash-report.txt` | Key segments from macOS crash reports (redacted) |
| `README.txt` | Instructions |

All API keys, tokens, email addresses, and home directory paths are automatically redacted. Review the files before uploading.

Then open a [bug report](https://github.com/sasuke39/open-warp/issues/new?template=bug_report.yml) and paste `issue-summary.md` into the diagnostics field.

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
