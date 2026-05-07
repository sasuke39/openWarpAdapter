# 故障排查

## macOS 提示应用已损坏

当前应用未签名。如果通过浏览器下载，macOS 可能会添加隔离标记。

```bash
xattr -cr /Applications/WarpLocal.app
```

## 设置页打不开

检查本地适配器是否正在监听：

```bash
curl http://127.0.0.1:18888/health
```

如果没有响应，完整退出 `WarpLocal.app` 后重新打开。

## AI 请求出现服务商错误

打开：

```text
http://127.0.0.1:18888/settings
```

确认：

- 接口地址是否正确
- 接口密钥是否已填写
- 模型名称是否被服务商支持
- Ollama、LM Studio 等本地服务是否已经启动

## 工具调用重复执行

请使用最新发布包。旧版本曾经存在过历史清理过早的问题，可能导致工具结果返回后模型仍然重复执行同一个命令。

## 打开 WarpLocal 后官方 Warp 退出

请使用最新发布包。`WarpLocal.app` 设计上可以和官方 Warp 并存。
