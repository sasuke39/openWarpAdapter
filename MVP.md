# Warp 本地 LLM 适配器 — MVP 实现文档

## 项目目标

绕过 Warp 云端后端，用自定义 API key 直连 LLM 厂商，保留 Warp 终端的 AI agent 能力（流式输出、工具调用）。

## 核心架构

```
Warp Client (改 server URL) → localhost:18888/ai/multi-agent → Go 本地后端 → OpenAI-compatible LLM API
```

Warp 客户端发 protobuf 编码的 `Request`，后端解码后调用 LLM，把 LLM 的响应翻译成 Warp 客户端期望的 SSE + base64 + protobuf `ResponseEvent` 流。

## 设计决策记录

| # | 决策点 | 选择 | 理由 |
|---|--------|------|------|
| 1 | LLM 接入方式 | OpenAI-compatible 兼容层 | 覆盖面最广（Ollama/LM Studio/vLLM/llama.cpp server 都暴露此接口） |
| 2 | 请求路由 | 客户端直连本地后端 | 本地服务云端不可达；隐私和延迟优势 |
| 3 | 实现方式 | 本地最小 HTTP/SSE 后端 | 客户端改动最小，只改请求目标地址 |
| 4 | 语言 | Go | Warp proto 是 Go 生态的，编译零问题；单二进制部署；和 Warp 同语言 |
| 5 | 客户端改造 | 改源码 server URL + 重编译 | 最直接可控；已有源码 |
| 6 | 工具调用机制 | OpenAI function calling | 主流本地服务都支持；结构化解析可靠 |
| 7 | 对话历史 | 本地后端内存维护 | MVP 最快；进程重启丢历史可接受 |
| 8 | System prompt | 自己写 | 原版是服务端动态生成（/harness-support/resolve-prompt），客户端代码里没有 |
| 9 | MVP 功能范围 | 聊天 + 只读工具 | 验证核心管道；零风险；写工具后续加 |
| 10 | 配置方式 | config.yaml | 简单直接；本地后端完全自主控制 |
| 11 | 客户端改造策略 | 直接改代码试 + 读代码预判 | 有源码可预判问题；抓包反而更麻烦 |
| 12 | 初始选 Python → 换 Go | proto 兼容性问题 | edition=2023 + go_features.proto 在 Python 生态不可用；Go 天然兼容 |

## Warp 后端职责分析（需要本地替代的能力）

Warp 后端不是单纯的模型代理，它承担了：

1. **协议入口层** — 接收 `warp_multi_agent_api::Request`（protobuf 编码）
2. **模型路由层** — 选择 provider / model / auth
3. **Agent 编排层** — 把模型输出变成 tool call / 文本；处理多轮工具回灌
4. **事件流编码层** — 产出 `Init` / `ClientActions` / `Finished` 三件套
5. **状态与计费层** — token usage、request cost、error reason mapping

MVP 只需覆盖主链路：1 → 2 → 3 → 4（5 简化处理）。

## 协议格式

### 请求
- **端点**: `POST /ai/multi-agent`
- **Content-Type**: `application/x-protobuf`
- **认证**: `Authorization: Bearer <token>`（本地模式可忽略）
- **请求体**: protobuf 编码的 `warp.multi_agent.v1.Request`

### 响应
- **格式**: SSE (text/event-stream)
- **每个事件**: `data: "<base64-url-safe-no-pad 编码的 protobuf ResponseEvent>"`
- **三大事件**:
  1. `StreamInit` — `conversation_id`, `request_id`, `run_id`
  2. `ClientActions` — 包含 `Message`（AgentOutput 文本 / ToolCall 工具调用）
  3. `StreamFinished` — 结束原因（Done/QuotaLimit/InvalidApiKey/InternalError 等）+ token usage

### 工具循环
```
请求1: 用户问题 → 后端调 LLM → 返回文本 + ToolCall → 客户端执行工具
请求2: ToolCallResult → 后端把结果喂给 LLM → 可能又一个 ToolCall → 客户端再执行
请求N: → LLM 返回最终文本 → Finished
```
每次请求是独立的 HTTP POST，对话历史由后端内存维护。

## MVP 功能范围

### 已包含
- 单会话流式文本输出
- OpenAI-compatible provider（自定义 base_url / api_key / model）
- 3 个只读工具：`read_files`、`grep`、`file_glob`
- 工具调用循环（LLM → tool_call → 客户端执行 → result → LLM 继续）
- Warp 客户端兼容的 SSE + protobuf 响应流

### 不包含（后续阶段）
- `run_shell_command` / `apply_file_diffs`（写操作 + 安全审批）
- Anthropic 原生 API 支持
- MCP 支持
- 子 agent / 编排
- 被动建议 (passive suggestions)
- 对话历史持久化（SQLite）
- 完整的 usage / cost 统计
- computer use

## 目录结构

```
local-adapter/
├── config.yaml              # LLM 配置（provider/base_url/api_key/model）
├── go.mod / go.sum          # Go 模块
├── proto/                   # 原始 .proto 文件（从 warp-proto-apis 下载）
├── proto3/                  # 转换后的 proto3 兼容版本
├── internal/
│   ├── config/config.go     # YAML 配置加载 ✅
│   ├── proto/               # 编译后的 Go protobuf 代码 ✅
│   ├── agent/prompt.go      # System prompt ✅
│   ├── tools/tools.go       # 3 个只读工具执行器 ✅
│   └── llm/llm.go           # OpenAI-compatible LLM 客户端 ⚠️ 编译中
└── cmd/server/              # HTTP 服务器入口（待实现）
```

## 已完成的工作

### 1. Proto 文件获取与编译 ✅
- 从 GitHub `warpdotdev/warp-proto-apis`（revision `78a78f21`）下载了全部 14 个 .proto 文件
- 原始文件使用 `edition = "2023"` 和 Go 专有 `go_features.proto`，不兼容标准 protoc
- 转换为 proto3 兼容格式（去除 Go 专有 import/option，修复 reserved 语法）
- 使用 `protoc-gen-go` 成功编译为 Go 代码
- 所有 14 个 `*.pb.go` 文件已生成在 `internal/proto/`

### 2. Go 项目初始化 ✅
- `go mod init`、依赖安装（`google.golang.org/protobuf`、`gopkg.in/yaml.v3`、`github.com/openai/openai-go`）
- 目录结构创建

### 3. 配置加载器 ✅
- `internal/config/config.go` — 读取 `config.yaml`

### 4. System Prompt ✅
- `internal/agent/prompt.go` — 描述 3 个只读工具的使用方式

### 5. 工具执行器 ✅
- `internal/tools/tools.go` — `ReadFiles`、`Grep`、`FileGlob` 三个函数

## 未完成的工作

### 6. LLM 客户端 ⚠️ 需要修复
- `internal/llm/llm.go` 已写，但 `openai-go` v1.12.0 的 API 和文档示例不完全一致
- **已知问题**:
  - `openai.F()` 不存在，需要用 `openai.String()` / `openai.Bool()` 或直接赋值
  - `param.Opt[T]` 需要类型参数，如 `param.Opt[string]("value")` 或 `param.NewOpt("value")`
  - `ChatCompletionNewParams` 没有 `Stream` 字段 — `NewStreaming()` 本身就是流式
  - 工具定义用 `openai.ChatCompletionToolParam` + `shared.FunctionDefinitionParam`
  - 工具描述字段用 `param.Opt[string]`，需要 `param.NewOpt("value")` 或 `param.Opt("value")`
- **修复方案**: 根据源码中发现的实际类型（`param.NewOpt[T]()`、`shared.FunctionDefinitionParam`、`shared.FunctionParameters`）修正代码

### 7. HTTP 服务器 ❌ 未开始
- `cmd/server/main.go` — 启动 HTTP 服务
- `POST /ai/multi-agent` 端点
- 解析 protobuf Request 请求体
- 编码 SSE + base64 + protobuf ResponseEvent 响应流
- 需要处理：CORS、错误处理、graceful shutdown

### 8. Agent 工具循环 ❌ 未开始
- 核心逻辑：LLM 流式响应 → 如果有 tool_call → 发 ClientActions 给客户端 → 客户端执行后回传 ToolCallResult → 喂回 LLM → 重复
- 对话历史管理（`conversation_id → []Message` 内存 map）
- 将 LLM 输出翻译为 Warp `ResponseEvent`：
  - 文本 → `AddMessagesToTask` + `AgentOutput`
  - tool_call → `AddMessagesToTask` + `ToolCall`
  - 结束 → `StreamFinished`（Done reason + 简化 token usage）

### 9. Warp 客户端改造 ✅
- 改 `server_api.rs` 中的 server base URL 为 `localhost:18888` ✅
- 处理 HTTP vs HTTPS（本地后端跑 HTTP）✅ — reqwest 默认支持 HTTP
- 跳过 Firebase 认证（本地模式不需要）✅ — Channel::Local + skip_firebase_anonymous_user feature
- 处理其他 API 端点失败（模型列表、GraphQL 等）— 容错处理
- 编译 Warp 客户端 ✅ — `cargo build --bin warp-oss -F skip_firebase_anonymous_user`

**修改的 3 个文件：**
1. `crates/warp_core/src/channel/config.rs` — 新增 `WarpServerConfig::local_adapter()` 返回 `http://127.0.0.1:18888`
2. `app/src/bin/oss.rs` — `Channel::Oss` → `Channel::Local`，使用 `local_adapter()` 配置
3. `app/src/server/server_api.rs` — `generate_multi_agent_output` 中 Local channel 跳过 Firebase auth，直接使用 `AuthToken::NoAuth`

**构建产物：** `target/debug/warp-oss`

### 10. 端到端测试 ❌ 待进行
- 启动 Go 后端 ✅ — 已验证启动成功，`/health` 返回 ok
- 启动改造后的 Warp 客户端 — 待用户执行
- 发送聊天消息，验证流式文本输出 — 待测试
- 测试工具调用（让模型读文件 / grep）— 待测试
- 验证工具结果回传和模型继续推理 — 待测试
- 验证流正常结束 — 待测试

**前置条件：** 用户需在 `config.yaml` 中填入有效的 API key

## 关键代码入口（Warp 源码）

- 请求组装: `app/src/ai/agent/api/impl.rs`
- 请求参数: `app/src/ai/agent/api.rs`
- 发往后端: `app/src/server/server_api.rs` (line 1091)
- 事件流消费: `app/src/ai/blocklist/controller.rs` (line 2271)
- Init 初始化: `app/src/ai/agent/conversation.rs` (line 1492)
- ClientActions 落地: `app/src/ai/blocklist/history_model.rs` (line 1275)
- Finished 处理: `app/src/ai/blocklist/controller.rs` (line 2580)
- 工具动作类型: `crates/ai/src/agent/action/mod.rs`
- 协议到动作转换: `crates/ai/src/agent/action/convert.rs`
- Proto 定义: GitHub `warpdotdev/warp-proto-apis` revision `78a78f21`

## 风险与注意事项

1. **Proto 版本同步**: Warp 更新 proto 后，本地后端需要同步。建议锁定 revision `78a78f21`。
2. **openai-go API 不稳定**: v1.12.0 的 API 和文档/示例不完全一致，需要以源码为准。
3. **Warp 客户端其他 API 调用**: 客户端启动后会调多个端点（模型列表、用户信息等），本地后端未实现这些端点，需要客户端容错。
4. **HTTPS 强制**: Warp 客户端可能强制 HTTPS，需要改代码允许 HTTP 连接 localhost。
5. **认证**: Warp 客户端带 Firebase Bearer token，本地后端需跳过验证，客户端需处理无认证场景。
6. **消息 ID 生成**: Warp 客户端期望 `Message.id` 和 `Task.id` 是唯一标识，需要生成策略（UUID）。
7. **Timestamp**: `Message.timestamp` 是 protobuf Timestamp，需要正确填充。
