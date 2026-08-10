build:
	wails build

dev:
	wails dev

test:
	go test ./...

lint:
	golangci-lint run

install-hooks:
	git config core.hooksPath .githooks && chmod +x .githooks/pre-commit