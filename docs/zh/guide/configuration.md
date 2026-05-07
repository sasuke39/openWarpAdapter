# 配置说明

运行时配置保存在本机。打包应用默认路径是：

```text
~/Library/Application Support/WarpLocal/config.yaml
```

正常使用时，打开 `WarpLocal.app`，在应用菜单中点击 `Local Adapter Settings...` 即可修改配置。菜单背后仍然是本地网页设置端点，但用户通常不需要手动访问。

## 配置示例

```yaml
provider: openai-compatible
base_url: https://api.openai.com/v1
api_key: YOUR_API_KEY
model: gpt-4.1-mini
server:
  host: 127.0.0.1
  port: 18888
```

## 服务商示例

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

## 健康检查

本地适配器提供几个简单接口：

```text
GET /health
GET /settings/status
POST /settings/reload
```

这些接口适合做冒烟测试和打包验证。
