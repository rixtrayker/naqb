## nqb — Makefile
##
## Usage:
##   make build        Build the nqb binary to ./bin/nqb
##   make install      Build and install to /usr/local/bin/nqb
##   make check        Gate check: build + vet + test (run before every commit)
##   make test         Run all tests
##   make test-v       Run all tests with verbose output
##   make test-race    Run all tests with race detector
##   make cover        Run tests and open coverage report in browser
##   make cover-text   Run tests and print per-package coverage summary
##   make vet          Run go vet
##   make lint         Run golangci-lint (requires: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
##   make tidy         Run go mod tidy
##   make clean        Remove built artifacts
##   make run          Build and run nqb (opens TUI home screen)
##   make debug        Build and run with NQB_DEBUG=1 (verbose log to stderr + ~/.naqb/nqb.log)
##   make log          Tail the nqb log file
##   make log-clear    Truncate the nqb log file

BINARY   := nqb
BIN_DIR  := ./bin
PKG      := ./cmd/nqb
LOG_FILE := $(HOME)/.naqb/nqb.log

# Build flags
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-X github.com/amr/naqb/internal/commands.version=$(VERSION) -s -w"

.PHONY: all build install check test test-v test-race cover cover-text vet lint tidy clean run debug log log-clear help

all: build

## build: Compile nqb binary to ./bin/nqb
build:
	@mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) $(PKG)
	@echo "Built → $(BIN_DIR)/$(BINARY)"

## install: Build and install nqb to /usr/local/bin
install: build
	cp $(BIN_DIR)/$(BINARY) /usr/local/bin/$(BINARY)
	@echo "Installed → /usr/local/bin/$(BINARY)"

## check: Gate check — build + vet + test all workspace modules (run before every commit)
check:
	go build ./...
	go vet ./...
	go test ./...
	@for mod in pkg/*/; do \
		echo "→ $$mod"; \
		(cd "$$mod" && go build ./... && go vet ./... && go test ./...) || exit 1; \
	done
	@echo "✓ build, vet, and tests all passed"

## test: Run all tests
test:
	go test ./...
	@for mod in pkg/*/; do \
		(cd "$$mod" && go test ./...) || exit 1; \
	done

## test-v: Run all tests with verbose output
test-v:
	go test -v ./...
	@for mod in pkg/*/; do \
		(cd "$$mod" && go test -v ./...) || exit 1; \
	done

## test-race: Run all tests with race detector
test-race:
	go test -race ./...
	@for mod in pkg/*/; do \
		(cd "$$mod" && go test -race ./...) || exit 1; \
	done

## cover: Run tests with coverage and open report
cover:
	go test -coverprofile=coverage.out ./...
	@for mod in pkg/*/; do \
		name=$$(basename "$$mod"); \
		(cd "$$mod" && go test -coverprofile=coverage-$$name.out ./...) || exit 1; \
	done
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report → coverage.html"
	open coverage.html 2>/dev/null || xdg-open coverage.html 2>/dev/null || echo "Open coverage.html in your browser"

## cover-text: Run tests with coverage and print per-package summary
cover-text:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | grep -E "(total|[0-9]+\.[0-9]+%)"

## vet: Run go vet
vet:
	go vet ./...
	@for mod in pkg/*/; do \
		(cd "$$mod" && go vet ./...) || exit 1; \
	done

## lint: Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
lint:
	golangci-lint run ./...
	@for mod in pkg/*/; do \
		(cd "$$mod" && golangci-lint run ./...) || exit 1; \
	done

## tidy: Synchronize workspace modules
tidy:
	go work sync

## clean: Remove build artifacts and coverage files
clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html

## run: Build and run nqb (opens TUI home screen)
run: build
	$(BIN_DIR)/$(BINARY)

## debug: Build and run with NQB_DEBUG=1 (logs to stderr + ~/.naqb/nqb.log)
debug: build
	NQB_DEBUG=1 $(BIN_DIR)/$(BINARY)

## log: Tail the nqb log file (live)
log:
	@mkdir -p $(HOME)/.naqb
	@touch $(LOG_FILE)
	tail -f $(LOG_FILE)

## log-clear: Truncate the nqb log file
log-clear:
	@truncate -s 0 $(LOG_FILE) 2>/dev/null || > $(LOG_FILE)
	@echo "Log cleared: $(LOG_FILE)"

## help: Show this help message
help:
	@echo "nqb — available make targets:"
	@grep -E '^## ' Makefile | sed 's/^## /  /'
