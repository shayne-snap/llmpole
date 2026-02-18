# Contributing to llmpole

Thanks for your interest in contributing.

## Reporting issues

Open an [issue](https://github.com/shayne-snap/llmpole/issues) with a clear description, steps to reproduce (if applicable), and your environment (OS, Go version).

## Sending changes

1. Fork the repo and create a branch from `main`.
2. Make your changes. Keep commits focused.
3. Run tests and build:
   ```bash
   go build ./...
   go test ./...
   ```
4. Open a pull request. Describe what changed and why; reference any related issues.

## Local setup

```bash
git clone https://github.com/shayne-snap/llmpole.git
cd llmpole
go build -o llmpole ./cmd/llmpole
./llmpole --help
```

No special configuration required. The app uses an embedded model list and optionally a user cache under your config directory.
