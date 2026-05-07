---
layout: home

hero:
  name: open-warp
  text: 在 Warp 里使用你自己的 AI 后端。
  tagline: 一个本地开源适配器，让 Warp AI 请求连接 DeepSeek、OpenAI、Ollama、OpenRouter、LM Studio、vLLM 等 OpenAI 兼容接口。
  actions:
    - theme: brand
      text: 快速开始
      link: /zh/guide/getting-started
    - theme: alt
      text: 查看 GitHub
      link: https://github.com/sasuke39/open-warp

features:
  - title: 本地优先
    details: WarpLocal 会把 AI 请求路由到本机 Go 辅助服务，而不是官方 Warp 云端后端。
  - title: 自定义模型服务
    details: 通过本地网页设置页配置服务商、接口地址、接口密钥和模型。
  - title: 面向编码工作流
    details: 已支持读文件、搜代码、运行命令、读取长命令输出、应用文件改动等核心工具。
  - title: 可与官方 Warp 并存
    details: WarpLocal 使用独立本地通道，不影响官方 Warp 应用。
---

## open-warp 是什么？

Warp 开源了客户端，但 AI 后端仍然由官方云服务控制。`open-warp` 补上的是本地 AI 服务端能力。

项目包含两部分：

- `WarpLocal.app`：打过补丁的 Warp 应用包，会把 AI 请求发送到 `127.0.0.1:18888`。
- `warp-local-adapter`：Go 编写的本地后端，把 Warp protobuf 流转换为 OpenAI 兼容接口请求。

## 当前状态

`open-warp` 已经可以覆盖核心编码链路：提问、读文件、搜索、跑命令、应用文件改动、继续处理工具结果。它目前不追求完整复刻 Warp 官方后端。

如果这个项目对你有帮助，欢迎给 [sasuke39/open-warp](https://github.com/sasuke39/open-warp) 一个收藏，这会帮助我们继续完善更多工具和发布体验。
