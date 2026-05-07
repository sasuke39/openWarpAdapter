# 快速开始

`open-warp` 通过 `WarpLocal.app`，让你在 Warp 里使用自己的 OpenAI 兼容大语言模型服务。

## 从发布包安装

从 GitHub 发布页下载最新的 `WarpLocal.app.zip`：

https://github.com/sasuke39/open-warp/releases

解压后，把 `WarpLocal.app` 移动到 `/Applications` 并打开。

如果 macOS 提示应用已损坏，清除隔离标记：

```bash
xattr -cr /Applications/WarpLocal.app
```

安装脚本也会自动处理这一步：

```bash
sh ./install.sh
```

## 配置模型服务

打开 `WarpLocal.app`，然后在应用菜单中点击 `Local Adapter Settings...`。

在设置界面中填写：

- 服务商名称
- 接口地址
- 接口密钥
- 模型名称

保存后，本地适配器会热重载配置，不需要完整重启应用。

本地 HTTP 设置地址仍然保留，主要用于调试；正常用户不需要手动打开。

## 在 WarpLocal 中使用 AI

打开 `WarpLocal.app`，按 `Cmd+K`，输入自然语言问题即可。

示例：

```text
分析当前目录
找一下服务端入口
创建一个简单的 Go HTTP 接口并运行测试
```

中文、日文、韩文输入会被识别为自然语言，不会直接落到 shell 里执行。
