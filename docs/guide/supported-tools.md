# Supported Tools

`open-warp` currently focuses on the core coding-agent loop.

## Available tools

| Tool | Purpose |
| --- | --- |
| `read_files` | Read selected files into context. |
| `grep` | Search file contents. |
| `file_glob` | Find files by glob patterns. |
| `file_glob_v2` | Find files with the newer Warp glob result shape. |
| `run_shell_command` | Ask WarpLocal to run shell commands through the client. |
| `read_shell_command_output` | Continue reading long-running command output. |
| `transfer_shell_command_control_to_user` | Hand a long-running command back to the user. |
| `apply_file_diffs` | Apply file changes through Warp's file-diff protocol. |
| `search_codebase` | Search across the current codebase. |

## Not supported yet

- MCP tools
- subagents
- computer use
- passive suggestions
- full Warp backend parity

Unsupported tool calls are rejected clearly instead of being sent to the Warp client as half-implemented actions.
