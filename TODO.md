# local-adapter 待修复问题

## 已完成：问题1 Tool result 内容提取（关键）

**现状**：`extractToolResult` 对整个 `tc.GetResult()` 做了 `json.Marshal`，发给 DeepSeek 的是原始 proto JSON：
```json
{"RunShellCommand":{"command":"ls","Result":{"CommandFinished":{"output":"...","command_id":"..."}}}}
```
DeepSeek 不理解 Warp 的内部 proto 包装格式。

**修复**：`cmd/server/main.go:263-270` 的 `extractToolResult` 函数，需要从 `Message_ToolCallResult` 的 Result oneof 中提取实际可读内容。至少处理以下类型：
- `RunShellCommand` → 提取 `CommandFinished.output` 或 `LongRunningCommandSnapshot` 或 `PermissionDenied`
- `ReadFiles` → 提取文件内容
- `Grep` → 提取匹配结果
- `FileGlob` / `FileGlobV2` → 提取匹配的文件列表
- 其他类型 → 给出有意义的文字描述

相关 proto 结构在 `internal/proto/task.pb.go`：
- `Message_ToolCallResult` struct (line 7099)，Result oneof 包含所有 tool result 变体
- `RunShellCommandResult` struct (line 1357)，Result oneof 包含 `CommandFinished` / `LongRunningCommandSnapshot` / `PermissionDenied`

## 已完成：问题2 Follow-up 请求复用 request_id 和 run_id

**现状**：每次请求后端都生成新的 `requestID` 和 `runID`。但 follow-up 请求（tool result 回来后）应该复用第一次请求的 ID，否则客户端的 exchange 可能关联不上。

**修复**：在 `Conversation` struct 中存储 `LastRequestID` 和 `LastRunID`。当 `isFollowUp` 为 true 时，`StreamInit` 使用存储的 ID 而不是生成新 UUID。参考 `cmd/server/main.go:149-191`。

## 已完成：问题3 Conversation 上限 30 个

**现状**：`Server.conversations` map 只增不减，长时间运行会内存泄漏。

**修复**：在 `getOrCreateConversation`（`main.go:50-61`）中添加淘汰逻辑：当 map 大小超过 30 时，删除最旧的条目。Conversation 需要记录创建时间（`CreatedAt time.Time`）用于判断新旧。

## 已完成：问题4 Conversation 持久化

**现状**：重启服务器后所有对话历史丢失。

**修复方案**：
1. 定义持久化文件路径（如 `conversations.json`，放在 config.yaml 同目录）
2. 服务器启动时从文件加载
3. 每次对话更新（添加消息后）写入文件
4. 优雅关闭时也写入一次
5. 文件格式：`{"conversation_id": {"history": [...], "last_request_id": "...", "last_run_id": "...", "created_at": "..."}}`

## 当前代码状态（已完成）

- DeepSeek reasoning_content 往返传递：修复完成，`CollectStreamResult` 提取 reasoning，`MakeAssistantMessageWithReasoning` 和 `MakeAssistantToolCallMessage` 带回
- ToolMessage 参数顺序：修复完成（`ToolMessage(content, toolCallID)` 顺序正确）
- Tool call 后发送 StreamFinished{Done}：修复完成
- Follow-up 请求跳过 CreateTask：修复完成（`isFollowUp` 参数控制）
- 6 个工具定义：read_files, grep, file_glob, file_glob_v2, run_shell_command, apply_file_diffs, search_codebase
- 工具 proto 序列化：RunShellCommand, ApplyFileDiffs, FileGlobV2, SearchCodebase 均已实现

## 测试方法

```bash
cd local-adapter
go run ./cmd/server/
```

Warp 客户端用 `warplocal://` scheme 启动，会自动连接 `http://127.0.0.1:18888/ai/multi-agent`。
