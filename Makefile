build:
	wails build -tags webkit2_41

dev:
	wails dev -tags webkit2_41

test:
	go test ./...

lint:
	golangci-lint run

install-hooks:
	git config core.hooksPath .githooks && chmod +x .githooks/pre-commit