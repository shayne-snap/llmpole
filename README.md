# llmpole

[中文说明](README.zh-CN.md)

[![CI](https://github.com/shayne-snap/llmpole/actions/workflows/ci.yml/badge.svg)](https://github.com/shayne-snap/llmpole/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**LLM Pole** — find your pole-position models. A terminal tool that right-sizes LLM models to your system's RAM, CPU, and GPU. It detects your hardware, scores each model on quality, speed, fit, and context, and tells you which ones will actually run well on your machine. Ships with an interactive TUI (default) and a classic CLI mode. Supports multi-GPU setups, MoE architectures, dynamic quantization selection, and speed estimation.

## Installation

**One-line install** (downloads the latest release and installs to `/usr/local/bin` or `~/.local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/shayne-snap/llmpole/main/install.sh | sh
```

**From source** (requires [Go 1.24+](https://go.dev/dl/)):

```bash
go install github.com/shayne-snap/llmpole@latest
```

Ensure `$GOPATH/bin` or `$HOME/go/bin` is in your `PATH`.

## Usage

- **`--version`, `-v`** — print version and exit.
- **No arguments** — starts the interactive TUI to browse models that fit your system.
- **`--cli`** — use table output instead of TUI when running with no subcommand.
- **`--json`** — output results as JSON where supported.
- **`--limit`, `-n`** — limit number of results (e.g. `-n 10`).
- **`--perfect`** — show only models that perfectly match recommended specs.

### Commands

| Command        | Description |
|----------------|-------------|
| `system`       | Show system hardware (RAM, CPU, GPU). |
| `list`         | List all LLM models. |
| `pole`         | Pole/adaptation analysis: models that fit your system, sorted by score. |
| `search [query]` | Search models by name, provider, or size. |
| `info [model]` | Show detailed info and fit for a model. |
| `recommend`    | Top recommendations for your hardware (options: `--use-case`, `-n`). |
| `update-list`  | Download the latest model list to your cache. |

### Examples

```bash
llmpole                    # TUI
llmpole --cli              # Table view
llmpole system             # Hardware summary
llmpole pole -n 5          # Top 5 fits
llmpole search llama       # Search by name
llmpole recommend -n 3     # Top 3 recommendations
```

### Demo

**TUI** (default — interactive browser):

![TUI Demo](demo/tui.gif)

**CLI** (table output with `--cli` and subcommands):

![CLI Demo](demo/cli.gif)

## Requirements

- **Go**: 1.24+ for building from source.
- **Platforms**: Linux (x86_64, aarch64), macOS (x86_64, arm64).

## License

MIT. See [LICENSE](LICENSE).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to report issues, send PRs, and run the project locally.
