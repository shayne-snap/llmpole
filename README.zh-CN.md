# llmpole

[English](README.md)

[![CI](https://github.com/shayne-snap/llmpole/actions/workflows/ci.yml/badge.svg)](https://github.com/shayne-snap/llmpole/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**LLM Pole** — 为你的硬件找到最合适的模型。终端工具，根据本机 RAM、CPU、GPU 为 LLM 模型做「选型」：自动检测硬件，从质量、速度、适配度、上下文等维度打分，告诉你哪些模型能在本机跑得好。默认提供交互式 TUI，也支持传统 CLI 表格输出；支持多 GPU、MoE 架构、动态量化选择与速度估算。

## 安装

**一键安装**（下载最新 release，安装到 `/usr/local/bin` 或 `~/.local/bin`）：

```bash
curl -fsSL https://raw.githubusercontent.com/shayne-snap/llmpole/main/install.sh | sh
```

**从源码安装**（需 [Go 1.24+](https://go.dev/dl/)）：

```bash
go install github.com/shayne-snap/llmpole@latest
```

请确保 `$GOPATH/bin` 或 `$HOME/go/bin` 已加入 `PATH`。

## 使用

- **`--version` / `-v`** — 打印版本并退出。
- **无参数** — 启动交互式 TUI，浏览适配本机的模型。
- **`--cli`** — 无子命令时使用表格输出而非 TUI。
- **`--json`** — 在支持的场景下以 JSON 输出结果。
- **`--limit` / `-n`** — 限制结果数量（如 `-n 10`）。
- **`--perfect`** — 仅显示完全符合推荐配置的模型。

### 命令

| 命令 | 说明 |
|------|------|
| `system` | 显示本机硬件（RAM、CPU、GPU）。 |
| `list` | 列出所有 LLM 模型。 |
| `pole` | 适配分析：按分数排序、适配本机的模型列表。 |
| `search [关键词]` | 按名称、提供商或规模搜索模型。 |
| `info [模型]` | 查看某模型的详细信息和适配情况。 |
| `recommend` | 为本机推荐模型（可选：`--use-case`、`-n`）。 |
| `update-list` | 从远端下载最新模型列表到本地缓存。 |

### 示例

```bash
llmpole                    # 启动 TUI
llmpole --cli              # 表格视图
llmpole system             # 硬件概览
llmpole pole -n 5          # 前 5 个适配结果
llmpole search llama       # 按名称搜索
llmpole recommend -n 3     # 前 3 条推荐
```

## 运行要求

- **Go**：从源码构建需 1.24+。
- **平台**：Linux（x86_64、aarch64）、macOS（x86_64、arm64）。

## 许可证

MIT，见 [LICENSE](LICENSE)。

## 参与贡献

如何反馈问题、提交 PR 及本地运行项目，请见 [CONTRIBUTING.md](CONTRIBUTING.md)。
