# 已支持工具

`open-warp` 当前聚焦核心编码智能体链路。

## 可用工具

| 工具 | 作用 |
| --- | --- |
| `read_files` | 读取指定文件内容。 |
| `grep` | 搜索文件内容。 |
| `file_glob` | 通过通配模式查找文件。 |
| `file_glob_v2` | 兼容新版 Warp 文件匹配结果结构。 |
| `run_shell_command` | 通过客户端执行 shell 命令。 |
| `read_shell_command_output` | 继续读取长时间运行命令的输出。 |
| `transfer_shell_command_control_to_user` | 将长时间运行命令交还给用户控制。 |
| `apply_file_diffs` | 通过 Warp 文件差异协议应用文件改动。 |
| `search_codebase` | 在当前代码库中搜索。 |

## 暂未支持

- MCP 工具
- 子智能体
- 计算机操作
- 被动建议
- 完整复刻 Warp 官方后端

如果模型请求了未支持工具，本地适配器会明确拒绝，而不是把半实现的动作发给 Warp 客户端。
