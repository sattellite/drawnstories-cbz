MAINFILE := cmd/drawnstories-cbz/main.go
BINARYFILE := drawnstories-cbz

.PHONY: help build test lint run

.DEFAULT_GOAL := help

build: ## Build binary file from sources for all platforms
	@go mod tidy -e
	@mkdir -p build
	GOOS=darwin GOARCH=amd64 go build -o build/$(BINARYFILE)-darwin-amd64 $(MAINFILE)
	GOOS=darwin GOARCH=arm64 go build -o build/$(BINARYFILE)-darwin-arm64 $(MAINFILE)
	GOOS=linux GOARCH=amd64 go build -o build/$(BINARYFILE)-linux-amd64 $(MAINFILE)
	GOOS=linux GOARCH=arm64 go build -o build/$(BINARYFILE)-linux-arm64 $(MAINFILE)

test: ## Run tests
	go test -v -race ./...

lint: ## Run linter for sources
	golangci-lint run ./...

run: ## Run program with test url
	@go mod tidy -e
	go run $(MAINFILE) https://drawnstories.ru/comics/Oni-press/rick-and-morty

# Auto documented Makefile https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
