.PHONY: build test lint clean coverage run install-tools

# Build variables
BINARY_NAME=rivian-ls
BUILD_DIR=.
COVERAGE_FILE=coverage.txt

# Build the binary
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/rivian-ls

# Run tests with coverage
test:
	go test -v -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...

# Run tests with coverage report
coverage: test
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report generated at coverage.html"

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Run the application
run: build
	./$(BINARY_NAME)

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f $(COVERAGE_FILE)
	rm -f coverage.html
	go clean

# Install development tools
install-tools:
	@echo "Installing golangci-lint..."
	@which golangci-lint > /dev/null || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Run all checks (fmt, vet, lint, test)
check: fmt vet lint test
	@echo "All checks passed!"
