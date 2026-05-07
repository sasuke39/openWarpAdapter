# Configuration

Runtime configuration is stored locally. For bundled apps, the default path is:

```text
~/Library/Application Support/WarpLocal/config.yaml
```

You can edit settings from the web UI:

```text
http://127.0.0.1:18888/settings
```

## Example

```yaml
provider: openai-compatible
base_url: https://api.openai.com/v1
api_key: YOUR_API_KEY
model: gpt-4.1-mini
server:
  host: 127.0.0.1
  port: 18888
```

## Provider examples

### OpenAI

```yaml
base_url: https://api.openai.com/v1
model: gpt-4.1-mini
```

### DeepSeek

```yaml
base_url: https://api.deepseek.com
model: deepseek-chat
```

### Ollama

```yaml
base_url: http://127.0.0.1:11434/v1
model: qwen2.5-coder
```

### LM Studio

```yaml
base_url: http://127.0.0.1:1234/v1
model: local-model
```

## Health checks

The adapter exposes simple local endpoints:

```text
GET /health
GET /settings/status
POST /settings/reload
```

These endpoints are useful for smoke tests and packaging checks.
