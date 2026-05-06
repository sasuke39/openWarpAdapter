# Warp Local Adapter

一个本地运行、开源、兼容 Warp 协议的编程代理适配器，对接 OpenAI 兼容接口的 LLM 服务商。

本项目让经过修改的 Warp 客户端与本地 Go 服务通信，而非 Warp 官方的 AI 后端。本地服务接收 Warp 的 protobuf 请求格式，调用 OpenAI 兼容的模型接口，并将 Warp 兼容的 SSE/protobuf 事件流返回给客户端。

## 项目状态

当前仓库是一个专注于核心编码循环的 MVP：

- 读取代码
- 搜索代码
- 执行 shell 命令
- 等待长时间运行的 shell 命令
- 应用文件差异
- 返回流式助手输出

它并非 Warp 生产后端的完整替代品。

## 已实现功能

当前 MVP 支持的工具：

- `read_files`
- `grep`
- `file_glob`
- `file_glob_v2`
- `run_shell_command`
- `read_shell_command_output`
- `transfer_shell_command_control_to_user`
- `apply_file_diffs`
- `search_codebase`

当前服务行为：

- Warp 兼容的 `POST /ai/multi-agent`
- protobuf 请求解码
- SSE + base64 + protobuf 响应事件
- 对话历史持久化
- 长时间运行命令跟踪
- 不支持的工具会被明确拒绝并返回错误文本

## 尚未实现

当前 MVP 不包含：

- MCP 工具
- 子代理 / 多代理编排
- 计算机操作
- 文档专用工具
- 被动建议
- 完整的 Warp 服务端对等能力

如果模型请求了不支持的工具，本地适配器会明确拒绝，而不是将无效的工具调用转发给客户端。

## 仓库结构

```text
local-adapter/
├── cmd/server/                 # HTTP 入口
├── internal/agent/             # 系统提示词
├── internal/config/            # 配置加载
├── internal/llm/               # OpenAI 兼容 LLM 客户端
├── internal/proto/             # 生成的 Go protobuf 文件
├── internal/tools/             # 本地工具实现
├── proto/                      # 适配器使用的协议源文件
├── proto3/                     # 生成时使用的兼容副本
├── MVP.md                      # 实现说明与架构历史
├── TODO.md                     # 待办事项
├── WARP_CLIENT.md              # WarpLocal 客户端补丁与构建指南
├── assets/                     # 项目跟踪的应用图标资源
└── build_and_bundle.sh         # macOS WarpLocal 打包脚本
```

## 快速开始

### 1. 环境要求

- Go 1.22 或更新版本
- 一个修改过的 Warp 客户端，指向本适配器
- 一个 OpenAI 兼容接口，例如：
  - OpenAI
  - OpenRouter
  - Ollama
  - LM Studio
  - vLLM

### 2. 配置

复制示例配置文件：

```bash
cp config.example.yaml config.yaml
```

然后编辑 `config.yaml`：

```yaml
provider: openai-compatible
base_url: https://api.openai.com/v1
api_key: YOUR_API_KEY
model: gpt-4.1-mini
server:
  host: 127.0.0.1
  port: 18888
```

### 3. 运行适配器

```bash
go run ./cmd/server
```

健康检查：

```bash
curl http://127.0.0.1:18888/health
```

### 4. 构建对接本地适配器的 Warp 客户端

本仓库包含 Warp 客户端补丁文件。完整构建指南见 **[WARP_CLIENT.md](./WARP_CLIENT.md)**。

快速步骤：

```bash
# 对 warp 源码目录打全部补丁
for patch in patches/*.patch; do patch -p1 < "$patch"; done

# 编译
cargo build --bin warp -F skip_firebase_anonymous_user
```

补丁涵盖：

- **服务端 URL 重定向** — 将客户端指向 `http://127.0.0.1:18888`
- **认证跳过** — 绕过 Firebase 认证（本地适配器不需要）
- **中文输入识别** — 修复中文/日文/韩文输入被当成 shell 命令的问题
- **本地设置入口** — 在 WarpLocal 里暴露 Local Adapter 设置入口

辅助脚本 `build_and_bundle.sh` 会构建可直接运行的 `WarpLocal.app`：

- `Contents/MacOS/warp` 作为主应用入口
- `Contents/Helpers/warp-local-adapter` 作为本地 AI helper
- `assets/` 中跟踪的图标资源

### 5. 构建 macOS App

```bash
sh ./build_and_bundle.sh
open ./WarpLocal.app
```

### 6. 从 Release 安装

```bash
sh ./install.sh
```

## 配置说明

运行时配置从 `config.yaml` 加载。

字段说明：

- `provider`：本适配器使用的自由格式标签
- `base_url`：OpenAI 兼容 API 的基础 URL
- `api_key`：服务商的 API 密钥
- `model`：模型名称
- `server.host`：绑定主机
- `server.port`：绑定端口

公开仓库不应提交真实的 `config.yaml` 文件。使用 `config.example.yaml` 作为模板。

## 协议兼容性与来源

本项目保留了一组最小的 Warp 兼容协议文件，使本地适配器能够使用修改后的 Warp 客户端所期望的请求/响应格式进行通信。

- `proto/` 包含本适配器工作流使用的协议源文件
- `internal/proto/` 包含运行时使用的生成 Go 绑定

这些文件仅用于与本项目使用的客户端协议保持兼容。本仓库不是 Warp 官方后端镜像，也不声称实现了完整的托管服务行为。

## 开发

格式化和测试：

```bash
gofmt -w ./cmd ./internal
go test ./...
```

## 本地设置页

WarpLocal 内置本地设置页：[http://127.0.0.1:18888/settings](http://127.0.0.1:18888/settings)。

这个页面主要负责：

- 配置 provider / base URL / API key / model
- 查看当前状态
- 保存配置并热重载

如果这个项目对你有帮助，欢迎去 [GitHub](https://github.com/sasuke39/openWarpAdapter) 点个 Star，算是给我们继续补更多 tool 能力的一点鼓励。

## 开源说明

本仓库有意排除以下内容：

- 本地对话状态
- 个人配置文件
- 编译后的二进制文件
- 本地应用打包

详见 `.gitignore`。

## 路线图

近期优先事项：

1. 让 `apply_file_diffs` 的失败报告更结构化
2. 改善长时间运行命令的行为
3. 添加 `ask_user_question` 作为后期 MVP 能力
4. 持续减少对不支持的 Warp 后端功能的意外依赖

## 开源协议

MIT。详见 [LICENSE](./LICENSE)。

## Star 历史

[![Star History Chart](https://api.star-history.com/svg?repos=sasuke39/openWarpAdapter&type=Date)](https://star-history.com/#sasuke39/openWarpAdapter&Date)
