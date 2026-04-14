.PHONY: build install test lint install-hooks

build:
	go build -o bin/athena ./cmd/athena

install:
	go install ./cmd/athena

test:
	go test ./...

lint:
	golangci-lint run

install-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit
	@echo "Pre-commit hook installed."