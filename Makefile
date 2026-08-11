build:
	wails build -tags webkit2_41

dev:
	wails dev -tags webkit2_41

test:
	go test ./...

lint:
	golangci-lint run

mutation-go:
	go run github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0 unleash ./internal/domain/... ./internal/application/...

mutation-frontend:
	cd frontend && npm run mutation

install-hooks:
	git config core.hooksPath .githooks && chmod +x .githooks/pre-commit .githooks/commit-msg