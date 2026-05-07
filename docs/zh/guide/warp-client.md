# Warp 客户端

`open-warp` 需要打过补丁的 Warp 客户端。仓库里的 `patches/` 目录包含这些补丁。

## 补丁做了什么

打补丁后的客户端会：

- 使用本地通道
- 把 AI 请求发送到 `http://127.0.0.1:18888`
- 本地适配器请求跳过 Firebase 认证
- 把中文、日文、韩文识别为自然语言输入
- 增加本地适配器设置菜单
- 本地构建跳过首次引导流程
- 应用退出时优雅停止本地适配器辅助进程

## 从源码构建

前置条件：

- Go 1.22 或更新版本
- Rust 工具链
- 兼容的 Warp 源码目录

构建：

```bash
git clone https://github.com/sasuke39/open-warp.git
cd open-warp
WARP_SRC=/path/to/warp-source sh ./build_and_bundle.sh
open ./WarpLocal.app
```

生成的应用包结构：

```text
WarpLocal.app/
└── Contents/
    ├── MacOS/warp
    ├── Helpers/warp-local-adapter
    └── Resources/
        ├── config.example.yaml
        └── iconfile.icns
```

更详细的补丁说明见仓库里的 `WARP_CLIENT.md`。
