.PHONY: build install test

build:
	go build -o ./bin/email-verifier ./cmd/verifier

install:
	go build -o $$(go env GOPATH)/bin/email-verifier ./cmd/verifier
	go build -o $$(go env GOPATH)/bin/email-verifer ./cmd/verifier
	go build -o $$(go env GOPATH)/bin/verifier ./cmd/verifier
	@if [ -d "/opt/homebrew/bin" ] && [ -w "/opt/homebrew/bin" ]; then \
		cp $$(go env GOPATH)/bin/email-verifier /opt/homebrew/bin/email-verifier; \
		cp $$(go env GOPATH)/bin/email-verifier /opt/homebrew/bin/email-verifer; \
		cp $$(go env GOPATH)/bin/email-verifier /opt/homebrew/bin/verifier; \
		echo "[SUCCESS] Installed to /opt/homebrew/bin"; \
	fi
	@echo "[SUCCESS] Installed email-verifier globally."

test:
	go test -v ./...
