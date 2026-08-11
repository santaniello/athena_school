build:
	wails build -tags webkit2_41

dev:
	wails dev -tags webkit2_41

test:
	go test ./...

lint:
	golangci-lint run

mutation-go:
	@for dir in internal/domain internal/application; do \
		if find "$$dir" -name '*.go' ! -name '*_test.go' | grep -q .; then \
			go run github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0 unleash ./$$dir || exit 1; \
		else \
			echo "No code yet in $$dir — skipping mutation testing."; \
		fi; \
	done

mutation-frontend:
	cd frontend && npm run mutation

mock:
	go run github.com/vektra/mockery/v2@v2.53.3

install-hooks:
	git config core.hooksPath .githooks && chmod +x .githooks/pre-commit .githooks/commit-msg