SHELL:=/bin/bash

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
APPLICATION := ci/slippy-find

.PHONY: lint
lint: install-tools
	@echo "Linting all modules..."
	@for dir in $(APPLICATION); do \
		if [ -d "$$dir" ]; then \
			echo "Linting $$dir module..."; \
			(cd $$dir && go mod tidy && golangci-lint run --config ../../.github/.golangci.yml --timeout 5m ./...); \
		else \
			echo "Directory $$dir not found, skipping..."; \
		fi; \
	done

.PHONY: test
test:
	@echo "Testing all modules..."
	@for dir in $(APPLICATION); do \
		if [ -d "$$dir" ]; then \
			echo "Testing $$dir module..."; \
			(cd $$dir && go mod download && go test -cover -coverprofile=coverage.out ./... && go tool cover -func coverage.out ); \
		else \
			echo "Directory $$dir not found, skipping..."; \
		fi; \
	done

.PHONY: clean
clean:
	@echo "Cleaning all modules..."
	@for dir in $(APPLICATION); do \
		if [ -d "$$dir" ]; then \
			echo "Cleaning $$dir module..."; \
			(cd $$dir && go clean ./... && go clean -testcache); \
		else \
			echo "Directory $$dir not found, skipping..."; \
		fi; \
	done

.PHONY: fmt
fmt: install-tools
	@echo "Formatting all modules..."
	@for dir in $(APPLICATION); do \
		if [ -d "$$dir" ]; then \
			echo "Formatting $$dir module..."; \
			(cd $$dir && golangci-lint fmt --config ../../.github/.golangci.yml ./...); \
		else \
			echo "Directory $$dir not found, skipping..."; \
		fi; \
	done

.PHONY: bump
bump:
	@echo "Bumping module versions..."
	@for dir in $(APPLICATION); do \
		if [ -d "$$dir" ]; then \
			echo "Bumping $$dir module..."; \
			(cd $$dir && go get -u && go mod tidy ); \
		else \
			echo "Directory $$dir not found, skipping..."; \
		fi; \
	done

.PHONY: tidy
tidy:
	@echo "Tidying up module dependencies..."
	@for dir in $(APPLICATION); do \
		if [ -d "$$dir" ]; then \
			echo "Tidying $$dir module..."; \
			(cd $$dir && go mod tidy ); \
		else \
			echo "Directory $$dir not found, skipping..."; \
		fi; \
	done

.PHONY: check-sec
check-sec:
	@echo "Checking security vulnerabilities in all modules..."
	@for dir in $(APPLICATION); do \
		if [ -d "$$dir" ]; then \
			echo "Checking $$dir module..."; \
			(cd $$dir && go mod download && go install golang.org/x/vuln/cmd/govulncheck@v1.1.4 && govulncheck -show verbose -test=false ./...) || exit 1; \
		else \
			echo "Directory $$dir not found, skipping..."; \
		fi; \
	done

.PHONY: install-tools
install-tools:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b `go env GOPATH`/bin v2.5.0




ACR_NAME := mycarrieracr
REPO_NAME := ci/slippy-find
DOCKERFILE_DIR := ci/slippy-find
# WORKFLOW_PATH := workflows/templates/slip-routing.yaml

# .PHONY: publish-test
# publish-test:
# 	@set -e; \
# 	az acr login -n $(ACR_NAME); \
# 	if [ -z "$(NEW_TAG)" ]; then \
# 		echo "Determining next tag..."; \
# 		EXISTING_TAGS=$$(az acr repository show-tags --name $(ACR_NAME) --repository $(REPO_NAME) --output tsv | grep -E 'test[0-9]+$$' || true); \
# 		LAST_NUM=$$(echo "$$EXISTING_TAGS" | sed "s/test//" | sort -nr | head -n1); \
# 		if [ -z "$$LAST_NUM" ]; then NEXT_NUM=1; else NEXT_NUM=$$((LAST_NUM + 1)); fi; \
# 		NEW_TAG="test$$NEXT_NUM"; \
# 		echo "Using new tag: $$NEW_TAG"; \
# 	else \
# 		echo "Using provided tag: $(NEW_TAG)"; \
# 	fi; \
# 	FULL_IMAGE="$(ACR_NAME).azurecr.io/$(REPO_NAME):$$NEW_TAG"; \
# 	echo "Building and publishing $$FULL_IMAGE for linux/amd64..."; \
# 	docker build --platform linux/amd64 -t $$FULL_IMAGE -f $(DOCKERFILE_DIR)/Dockerfile $(DOCKERFILE_DIR); \
# 	docker push $$FULL_IMAGE; \
# 	echo "Successfully published $$FULL_IMAGE"; \
# 	if [ -n "$$WORKFLOW_DEV_DIR" ]; then \
# 		WORKFLOW_FILE="$$WORKFLOW_DEV_DIR/$(WORKFLOW_PATH)"; \
# 		CWD=$$(pwd); \
# 		if [ -f "$$WORKFLOW_FILE" ]; then \
# 			echo "Updating image references in $$WORKFLOW_FILE..."; \
# 			sed -i '' "s|$(ACR_NAME).azurecr.io/$(REPO_NAME):[^ ]*|$$FULL_IMAGE|g" "$$WORKFLOW_FILE"; \
# 			echo "Updated workflow file with new image: $$FULL_IMAGE"; \
# 			cd $$WORKFLOW_DEV_DIR; \
# 			git add $(WORKFLOW_PATH); \
# 			git commit -m "Updated workflow file with new image: $$FULL_IMAGE"; \
# 			git push; \
# 			cd $$CWD; \
# 		else \
# 			echo "Warning: Workflow file not found at $$WORKFLOW_FILE"; \
# 		fi; \
# 	else \
# 		echo "WORKFLOW_DEV_DIR not set, skipping workflow file update"; \
# 	fi