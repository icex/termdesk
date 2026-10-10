BINARY_NAME=termdesk
BUILD_DIR=bin
GO=go
GOFLAGS=-v

.PHONY: all build run test test-coverage test-golden-update lint clean fmt vet test-e2e

all: build

build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/termdesk

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

test:
	$(GO) test ./... -v -race -count=1

test-e2e:
	python3 scripts/test-e2e.py

test-coverage:
	$(GO) test ./... -coverprofile=coverage.out -covermode=atomic
	$(GO) tool cover -func=coverage.out
	@echo "To view in browser: go tool cover -html=coverage.out"

test-golden-update:
	$(GO) test ./internal/app -run '^TestGolden' -update

lint: vet
	@which golangci-lint > /dev/null 2>&1 || echo "golangci-lint not installed, skipping"
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || true

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out
