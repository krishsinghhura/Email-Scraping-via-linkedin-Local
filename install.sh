#!/usr/bin/env bash
set -e

GOPATH_BIN="$(go env GOPATH)/bin"
mkdir -p "$GOPATH_BIN"

echo "[INFO] Compiling and installing email-verifier..."
go build -o "$GOPATH_BIN/email-verifier" ./cmd/verifier
go build -o "$GOPATH_BIN/email-verifer" ./cmd/verifier
go build -o "$GOPATH_BIN/verifier" ./cmd/verifier

if [ -d "/opt/homebrew/bin" ] && [ -w "/opt/homebrew/bin" ]; then
	cp "$GOPATH_BIN/email-verifier" /opt/homebrew/bin/email-verifier
	cp "$GOPATH_BIN/email-verifier" /opt/homebrew/bin/email-verifer
	cp "$GOPATH_BIN/email-verifier" /opt/homebrew/bin/verifier
	echo "[SUCCESS] Copied binaries to /opt/homebrew/bin"
fi

if ! grep -q 'go env GOPATH' ~/.zshrc 2>/dev/null; then
	echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.zshrc
fi

echo "[SUCCESS] Installed email-verifier globally."
echo "Usage: email-verifier -help"
