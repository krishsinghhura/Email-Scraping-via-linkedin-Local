#!/usr/bin/env bash
set -e

REPO_URL="https://github.com/krishsinghhura/Email-Scraping-via-linkedin-Local.git"
GOPATH_BIN="$(go env GOPATH 2>/dev/null || echo "$HOME/go")/bin"
mkdir -p "$GOPATH_BIN"

if ! command -v go >/dev/null 2>&1; then
	echo "[ERROR] Go is not installed. Please install Go (https://go.dev) to use email-verifier."
	exit 1
fi

TEMP_DIR=""
if [ ! -f "./cmd/verifier/main.go" ]; then
	echo "[INFO] Downloading repository..."
	TEMP_DIR=$(mktemp -d)
	git clone --depth 1 "$REPO_URL" "$TEMP_DIR" >/dev/null 2>&1
	cd "$TEMP_DIR"
fi

echo "[INFO] Compiling email-verifier..."
go build -o "$GOPATH_BIN/email-verifier" ./cmd/verifier
go build -o "$GOPATH_BIN/email-verifer" ./cmd/verifier
go build -o "$GOPATH_BIN/verifier" ./cmd/verifier

INSTALL_DIR=""
if [ -d "/opt/homebrew/bin" ] && [ -w "/opt/homebrew/bin" ]; then
	INSTALL_DIR="/opt/homebrew/bin"
elif [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
	INSTALL_DIR="/usr/local/bin"
fi

if [ -n "$INSTALL_DIR" ]; then
	cp "$GOPATH_BIN/email-verifier" "$INSTALL_DIR/email-verifier"
	cp "$GOPATH_BIN/email-verifier" "$INSTALL_DIR/email-verifer"
	cp "$GOPATH_BIN/email-verifier" "$INSTALL_DIR/verifier"
	echo "[SUCCESS] Copied binaries to $INSTALL_DIR"
fi

SHELL_RC=""
if [ -f "$HOME/.zshrc" ]; then
	SHELL_RC="$HOME/.zshrc"
elif [ -f "$HOME/.bashrc" ]; then
	SHELL_RC="$HOME/.bashrc"
fi

if [ -n "$SHELL_RC" ] && ! grep -q 'go env GOPATH' "$SHELL_RC" 2>/dev/null; then
	echo 'export PATH=$PATH:$(go env GOPATH 2>/dev/null || echo "$HOME/go")/bin' >> "$SHELL_RC"
fi

if [ -n "$TEMP_DIR" ]; then
	rm -rf "$TEMP_DIR"
fi

echo "[SUCCESS] Installation complete! You can now run:"
echo "  email-verifier -help"
