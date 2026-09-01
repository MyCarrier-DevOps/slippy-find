SHELL:=/bin/bash

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BINARY := slippy-find

# This module lives at the repository root (go.mod is at ./go.mod).
APPLICATION := .

GOLANGCI_LINT_VERSION := v2.13.2
GOVULNCHECK_VERSION   := v1.7.0
MUTEST_VERSION        := v0.6.0

MUTATION_BASE      ?= origin/main
MUTATION_THRESHOLD ?= 100

.PHONY: lint
lint: install-tools
	@echo "Linting module..."
	go mod tidy
	golangci-lint run --config .github/.golangci.yml --timeout 5m ./...

.PHONY: test
test:
	@echo "Running tests..."
	go mod download
	go test -race -cover -coverprofile=coverage.out ./...
	go tool cover -func coverage.out

.PHONY: clean
clean:
	@echo "Cleaning..."
	go clean ./...
	go clean -testcache
	rm -f $(BINARY) coverage.out

.PHONY: fmt
fmt: install-tools
	@echo "Formatting..."
	golangci-lint fmt --config .github/.golangci.yml ./...

.PHONY: bump
bump:
	@echo "Bumping module versions..."
	go get -u
	go mod tidy

.PHONY: tidy
tidy:
	@echo "Tidying up module dependencies..."
	go mod tidy

.PHONY: check-sec
check-sec:
	@echo "Checking security vulnerabilities..."
	go mod download
	go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	govulncheck -show verbose ./...

.PHONY: build
build:
	@echo "Building $(BINARY)..."
	go build -o $(BINARY) .

.PHONY: install-tools
install-tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: mutation
mutation:
	@echo "Mutation testing code changed vs $(MUTATION_BASE)..."
	@for dir in $(APPLICATION); do \
		if [ -d "$$dir" ]; then \
			echo "Mutation testing $$dir module..."; \
			(cd $$dir && { command -v mutest >/dev/null 2>&1 || go install github.com/fchimpan/mutest@$(MUTEST_VERSION); } && mutest -diff $(MUTATION_BASE) -threshold $(MUTATION_THRESHOLD) ./...) || exit 1; \
		fi; \
	done

.PHONY: mutation-all
mutation-all:
	@echo "Mutation testing all modules (threshold $(MUTATION_THRESHOLD)%)..."
	@for dir in $(APPLICATION); do \
		if [ -d "$$dir" ]; then \
			echo "Mutation testing $$dir module..."; \
			(cd $$dir && go mod download && { command -v mutest >/dev/null 2>&1 || go install github.com/fchimpan/mutest@$(MUTEST_VERSION); } && mutest -threshold $(MUTATION_THRESHOLD) ./...) || exit 1; \
		fi; \
	done

.PHONY: run
run:
	go run . $(ARGS)

.PHONY: ci
ci: fmt lint test build

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  make build          - build ./$(BINARY)"
	@echo "  make run ARGS=...   - run the CLI from source"
	@echo "  make test           - unit tests with race + coverage"
	@echo "  make mutation       - mutation-test code changed vs $(MUTATION_BASE) (mutest, $(MUTATION_THRESHOLD)% kill)"
	@echo "  make mutation-all   - mutation-test the whole module (weekly CI audit)"
	@echo "  make lint           - run golangci-lint"
	@echo "  make fmt            - format code via golangci-lint"
	@echo "  make check-sec      - run govulncheck"
	@echo "  make tidy           - go mod tidy"
	@echo "  make bump           - upgrade dependencies"
	@echo "  make clean          - clean build & test caches"
	@echo "  make install-tools  - install golangci-lint $(GOLANGCI_LINT_VERSION) locally"
	@echo "  make ci             - fmt + lint + test + build"
