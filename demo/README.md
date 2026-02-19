# Demo (VHS tapes)

Terminal demos are recorded with [VHS](https://github.com/charmbracelet/vhs). Each `.tape` file produces a GIF.

## Prerequisites

- [VHS](https://github.com/charmbracelet/vhs) (e.g. `brew install vhs`)
- [ttyd](https://github.com/tsl0922/ttyd) (e.g. `brew install ttyd`)
- `llmpole` in your PATH (from repo root: `go build -o llmpole ./cmd/llmpole` then `export PATH=$PWD:$PATH`)

## Record

From the **repository root**:

```bash
vhs demo/cli.tape   # → demo/cli.gif
vhs demo/tui.tape   # → demo/tui.gif
```

- **cli.tape** — CLI mode: `llmpole system`, `llmpole --cli -n 8`, `llmpole pole -n 5`
- **tui.tape** — Default TUI: start, j/k navigation, quit with `q`
