# WarpLocal

在 Warp 里使用你自己的大语言模型。这个本地开源适配器可以让打过补丁的 Warp 终端连接任意 OpenAI 兼容接口，包括 OpenAI、DeepSeek、Ollama、OpenRouter、LM Studio、vLLM 等。

**工作原理：** WarpLocal 对 Warp 客户端打补丁，将 AI 请求路由到本地 Go 服务，而不是 Warp 云端。这个本地服务会把 Warp 的 protobuf 协议转换成 OpenAI 兼容接口请求，执行已支持的本地工具，并把结果流式返回给客户端。

## 特性

- 支持任意 OpenAI 兼容接口，包括 OpenAI、DeepSeek、Ollama、OpenRouter、vLLM、LM Studio
- 开箱即用的 `WarpLocal.app`，双击即可启动，无需命令行
- 网页设置页面：`http://127.0.0.1:18888/settings`
- 中文输入支持，中文、日文、韩文会自动识别为 AI 查询
- 可与官方 Warp 并存使用

## 已支持工具

`read_files` · `grep` · `file_glob` · `file_glob_v2` · `run_shell_command` · `read_shell_command_output` · `transfer_shell_command_control_to_user` · `apply_file_diffs` · `search_codebase`

尚未支持：MCP 工具、子代理、计算机操作、被动建议。

## 安装

### 方式一：下载发布包（推荐）

```bash
sh ./install.sh
```

从 [GitHub 发布页](https://github.com/sasuke39/openWarpAdapter/releases) 下载最新的 `WarpLocal.app` 并安装。

> **macOS 提示应用已损坏？** 从浏览器直接下载的未签名应用可能会被系统拦截。运行以下命令清除隔离标记即可：
> ```bash
> xattr -cr /Applications/WarpLocal.app
> ```
> 使用 `sh ./install.sh` 安装会自动处理，不会出现此问题。

### 方式二：从源码构建

前置条件：Go 1.22+、Rust 工具链、Warp 源码（v0.2026.04.29）

```bash
# 1. 克隆本仓库
git clone https://github.com/sasuke39/openWarpAdapter.git
cd openWarpAdapter

# 2. 构建 WarpLocal.app
WARP_SRC=/path/to/warp-source sh ./build_and_bundle.sh
open ./WarpLocal.app
```

完整补丁与构建指南见 **[WARP_CLIENT.md](./WARP_CLIENT.md)**。

## 快速开始

1. **启动** `WarpLocal.app`
2. **打开设置**：应用菜单中有 `Local Adapter Settings...` 入口，也可以直接访问 [http://127.0.0.1:18888/settings](http://127.0.0.1:18888/settings)
3. **配置** 你的服务商、接口密钥和模型
4. **开始编程** — 在 WarpLocal 中按 `Cmd+K` 即可与大语言模型对话

## 配置

运行时配置存储在 `config.yaml`（打包应用为 `~/Library/Application Support/WarpLocal/config.yaml`）。

```yaml
provider: openai-compatible
base_url: https://api.openai.com/v1
api_key: YOUR_API_KEY
model: gpt-4.1-mini
server:
  host: 127.0.0.1
  port: 18888
```

也可以完全通过网页设置页面配置，无需手动编辑 YAML。

## 仓库结构

```text
├── cmd/server/                 # Go HTTP 服务器（本地适配器）
├── internal/agent/             # 系统提示词
├── internal/config/            # 配置加载
├── internal/llm/               # OpenAI 兼容模型客户端
├── internal/proto/             # 生成的 Go protobuf 文件
├── internal/tools/             # 本地工具实现
├── patches/                    # Warp 客户端补丁
├── assets/                     # 应用图标
├── build_and_bundle.sh         # macOS WarpLocal.app 打包脚本
├── install.sh                  # 一键安装脚本
├── WARP_CLIENT.md              # 完整补丁与构建指南
```

## Warp 客户端补丁

`patches/` 目录包含 Warp 客户端补丁：

| 补丁 | 作用 |
|------|------|
| 0001 | `WarpServerConfig::local_adapter()` — 将请求路由到 `127.0.0.1:18888` |
| 0002 | `Channel::Local` 入口 — 激活本地适配器配置 |
| 0003 | 跳过 Firebase 认证 — 本地适配器不需要云端认证 |
| 0004 | 中文自然语言检测 — 中文/日文/韩文输入识别为 AI 查询 |
| 0005 | Warp UI 中添加 "Local Adapter Settings..." 菜单项 |
| 0006 | 本地构建跳过首次引导流程 |
| 0007 | 退出 WarpLocal 时优雅停止本地适配器辅助进程 |

各补丁详情见 **[WARP_CLIENT.md](./WARP_CLIENT.md)**。

## 应用包结构

```
WarpLocal.app/
└── Contents/
    ├── MacOS/warp                # WarpLocal 主程序
    ├── Helpers/warp-local-adapter # Go AI 后端服务
    └── Resources/
        ├── config.example.yaml
        └── iconfile.icns
```

Warp 客户端会管理本地适配器服务的生命周期，自动启动辅助进程并保持运行。

## 开发

```bash
go test ./...
gofmt -w ./cmd ./internal
```

## 路线图

1. 原生 Warp 设置页面（替代网页设置界面）
2. `ask_user_question` 工具支持
3. 更好的 `apply_file_diffs` 失败报告
4. 改善长时间运行命令的行为
5. CI 自动化发布

## 收藏趋势

[![收藏趋势图](https://api.star-history.com/svg?repos=sasuke39/openWarpAdapter&type=Date)](https://star-history.com/#sasuke39/openWarpAdapter&Date)

## 开源协议

MIT。详见 [LICENSE](./LICENSE)。
