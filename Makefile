.PHONY: docs
docs:
	swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal

.PHONY: dev
dev:
	go run ./cmd/server

.PHONY: check
check:
	go mod tidy
	go test ./...
	golangci-lint run
	$(MAKE) docs