## nqb — Makefile
##
## Usage:
##   make build        Build the nqb binary to ./bin/nqb
##   make install      Build and install to /usr/local/bin/nqb
##   make test         Run all tests
##   make test-v       Run all tests with verbose output
##   make cover        Run tests and open coverage report in browser
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
LDFLAGS  := -ldflags "-X main.version=$(VERSION) -s -w"

.PHONY: all build install test test-v cover vet lint tidy clean run debug log log-clear help

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

## test: Run all tests
test:
	go test ./...

## test-v: Run all tests with verbose output
test-v:
	go test -v ./...

## test-race: Run all tests with race detector
test-race:
	go test -race ./...

## cover: Run tests with coverage and open report
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report → coverage.html"
	open coverage.html 2>/dev/null || xdg-open coverage.html 2>/dev/null || echo "Open coverage.html in your browser"

## vet: Run go vet
vet:
	go vet ./...

## lint: Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
lint:
	golangci-lint run ./...

## tidy: Run go mod tidy
tidy:
	go mod tidy

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
