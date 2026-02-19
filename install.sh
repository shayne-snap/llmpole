#!/bin/sh
# llmpole installer
# Usage: curl -fsSL https://raw.githubusercontent.com/shayne-snap/llmpole/main/install.sh | sh
#
# Downloads the latest llmpole release from GitHub and installs
# the binary to /usr/local/bin (or ~/.local/bin if no sudo).

set -e

REPO="shayne-snap/llmpole"
BINARY="llmpole"

# --- helpers ---

info() { printf '  \033[1;34m>\033[0m %s\n' "$*"; }
err()  { printf '  \033[1;31m!\033[0m %s\n' "$*" >&2; exit 1; }

need() {
    command -v "$1" >/dev/null 2>&1 || err "Required tool '$1' not found. Please install it and try again."
}

# --- detect platform ---

detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Linux)  OS="unknown-linux-musl" ;;
        Darwin) OS="apple-darwin" ;;
        *)      err "Unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        x86_64|amd64)   ARCH="x86_64" ;;
        aarch64|arm64)  ARCH="aarch64" ;;
        *)              err "Unsupported architecture: $ARCH" ;;
    esac

    PLATFORM="${ARCH}-${OS}"
}

# --- install dir ---

pick_install_dir() {
    NEED_SUDO=0
    if [ -w /usr/local/bin ]; then
        INSTALL_DIR="/usr/local/bin"
    elif command -v sudo >/dev/null 2>&1; then
        INSTALL_DIR="/usr/local/bin"
        NEED_SUDO=1
    else
        INSTALL_DIR="${HOME}/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi
}

# --- fetch latest release ---

fetch_latest_tag() {
    need curl

    # Note: `/releases/latest` returns 404 if there are no releases.
    JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null || true)"
    if [ -z "$JSON" ]; then
        TAG=""
        return 0
    fi

    TAG="$(printf '%s' "$JSON" \
        | grep '"tag_name"' \
        | head -1 \
        | sed 's/.*"tag_name": *"//;s/".*//')"

    [ -n "$TAG" ] || TAG=""
}

# --- download and install ---

install_release() {
    need tar
    pick_install_dir

    ASSET="${BINARY}-${TAG}-${PLATFORM}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

    TMPDIR="$(mktemp -d)"
    trap 'rm -rf "$TMPDIR"' EXIT

    info "Downloading ${BINARY} ${TAG} for ${PLATFORM}..."
    curl -fsSL "$URL" -o "${TMPDIR}/${ASSET}" \
        || err "Download failed. Asset '${ASSET}' may not exist for your platform.\n  Check: https://github.com/${REPO}/releases/tag/${TAG}"

    info "Extracting..."
    tar -xzf "${TMPDIR}/${ASSET}" -C "$TMPDIR"

    # Find the binary in the extracted contents
    BIN="$(find "$TMPDIR" -name "$BINARY" -type f | head -1)"
    [ -n "$BIN" ] || err "Binary not found in archive. Release asset may have an unexpected layout."
    chmod +x "$BIN"

    if [ "$NEED_SUDO" -eq 1 ]; then
        info "Installing to /usr/local/bin (requires sudo)..."
        sudo mv "$BIN" "${INSTALL_DIR}/${BINARY}"
    else
        mv "$BIN" "${INSTALL_DIR}/${BINARY}"
    fi

    info "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"

    # Check if install dir is in PATH
    case ":$PATH:" in
        *":${INSTALL_DIR}:"*) ;;
        *) info "Add ${INSTALL_DIR} to your PATH to use '${BINARY}' directly." ;;
    esac
}

install_from_source() {
    need go
    pick_install_dir

    PKG="github.com/${REPO}/cmd/${BINARY}@main"

    TMPDIR="$(mktemp -d)"
    trap 'rm -rf "$TMPDIR"' EXIT

    info "Building from source (${PKG})..."
    GOBIN="$TMPDIR" go install "$PKG" || err "Go build failed. Try installing Go or use a tagged version."

    BIN="${TMPDIR}/${BINARY}"
    [ -f "$BIN" ] || err "Built binary not found at ${BIN}."
    chmod +x "$BIN"

    if [ "$NEED_SUDO" -eq 1 ]; then
        info "Installing to /usr/local/bin (requires sudo)..."
        sudo mv "$BIN" "${INSTALL_DIR}/${BINARY}"
    else
        mv "$BIN" "${INSTALL_DIR}/${BINARY}"
    fi

    info "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"

    case ":$PATH:" in
        *":${INSTALL_DIR}:"*) ;;
        *) info "Add ${INSTALL_DIR} to your PATH to use '${BINARY}' directly." ;;
    esac
}

# --- main ---

main() {
    info "llmpole installer"
    detect_platform
    fetch_latest_tag
    if [ -n "$TAG" ]; then
        install_release
    else
        info "No GitHub releases found; installing from source (Go required)..."
        install_from_source
    fi
    info "Done. Run '${BINARY}' to get started."
}

main
