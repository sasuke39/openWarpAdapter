# 构建对接本地适配器的 Warp 客户端

GitHub 仓库只包含 **本地适配器与补丁**（`local-adapter`）。要让 Warp 客户端对接本地适配器，需要对 Warp 源码打 5 个补丁并重新编译。

## 前置条件

- [Rust 工具链](https://rustup.rs)（编译 Warp）
- Warp 源码 —— 本项目基于 `warp-v0.2026.04.29.08.56.stable_00` 版本开发
- 打好补丁后，Warp 客户端会连接 `http://127.0.0.1:18888`，不再依赖 Warp 云服务

## 补丁列表

`patches/` 目录下 5 个补丁，按顺序打：

| 序号 | 文件 | 作用 |
|------|------|------|
| 0001 | `crates/warp_core/src/channel/config.rs` | 新增 `local_adapter()` 配置，指向本地适配器 |
| 0002 | `app/src/bin/oss.rs` | 入口改用 `Channel::Local` + `local_adapter()` |
| 0003 | `app/src/server/server_api.rs` | 本地模式下跳过 Firebase 认证 |
| 0004 | `crates/natural_language_detection/src/lib.rs` | 中文等非拉丁文字识别为自然语言（修复中文输入被当成 shell 命令的问题） |
| 0005 | `app/src/app_menus.rs` 等 | 暴露 Local Adapter 设置入口 |

## 打补丁 + 编译

```bash
# 1. 进入 Warp 源码目录
cd warp/

# 2. 按顺序打补丁
for patch in ../local-adapter/patches/*.patch; do
  patch -p1 < "$patch"
done

# 3. 编译
cargo build --bin warp -F skip_firebase_anonymous_user

# 4. 产物位置
ls -lh target/debug/warp
```

## 补丁说明

### 0001 — 本地适配器配置

在 `WarpServerConfig` 中新增 `local_adapter()` 方法，将 Warp 的请求目标从云服务改为本地。

### 0002 — 入口切换

将 `oss.rs` 中的 channel 从 `Channel::Oss` 改为 `Channel::Local`，触发生效 `local_adapter()` 配置。

### 0003 — 跳过认证

本地适配器不验证 Firebase token。当 channel 为 `Channel::Local` 时，直接使用 `AuthToken::NoAuth`，不再调用 Firebase 认证流程。

这个改动是必需的，否则 Warp 客户端启动后会尝试向 `https://app.warp.dev` 做 Firebase 登录，而本地适配器没有这个端点。

### 0004 — 中文输入识别

Warp 的输入分类器（`input_classifier` crate）用英文词表判断输入是 shell 命令还是 AI 查询。中文不在词表中 → 被当成 shell 命令 → `zsh: command not found`。

这个补丁在 `natural_language_words_score()` 中增加非拉丁文字检测：CJK（中日韩）、西里尔、阿拉伯文等字符直接识别为自然语言 token。

### 0005 — Local Adapter 设置入口

在 WarpLocal 的 AI 菜单 / 设置里增加 Local Adapter 入口，直接打开本地设置页，方便修改 provider、模型和 API key。

## 本地适配器启动

```bash
cd local-adapter
cp config.example.yaml config.yaml   # 填入你的 API key 和模型
go run ./cmd/server                  # 启动在 127.0.0.1:18888
```

然后运行编译好的 `warp`，或者直接执行：

```bash
cd local-adapter
sh ./build_and_bundle.sh
open ./WarpLocal.app
```

之后在 WarpLocal 里用 Cmd+K 即可与本地 LLM 对话。
