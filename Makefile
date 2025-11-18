.PHONY: test test-coverage

# Default target
.DEFAULT_GOAL := help

# Run linter (requires golangci-lint)
lint:
	@echo "🔍 Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout 5m ./...; \
	else \
		echo "⚠️  golangci-lint not installed. Install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin"; \
		exit 1; \
	fi

# Format code
fmt:
	@echo "✨ Formatting code..."
	@go fmt ./...
	@echo "✅ Formatting complete"

# Check if code is formatted
fmt-check:
	@echo "🔍 Checking code formatting..."
	@test -z "$$(gofmt -l .)" || (echo "❌ Code is not formatted. Run 'make fmt'" && gofmt -l . && exit 1)
	@echo "✅ Code is properly formatted"

# Run go vet
vet:
	@echo "🔍 Running go vet..."
	@go vet ./...
	@echo "✅ Vet complete"

# Download dependencies
deps:
	@echo "📦 Downloading dependencies..."
	@go mod download
	@go mod verify
	@echo "✅ Dependencies downloaded"

# Update dependencies
deps-update:
	@echo "📦 Updating dependencies..."
	@go get -u ./...
	@go mod tidy
	@echo "✅ Dependencies updated"

# Tidy dependencies
deps-tidy:
	@echo "📦 Tidying dependencies..."
	@go mod tidy
	@echo "✅ Dependencies tidied"

# Show help
help:
	@echo "📚 Available targets:"
	@echo ""
	@echo "  Code quality:"
	@echo "    lint             - Run golangci-lint"
	@echo "    fmt              - Format code with gofmt"
	@echo "    fmt-check        - Check if code is formatted"
	@echo "    vet              - Run go vet"
	@echo ""
	@echo "  Dependencies:"
	@echo "    deps             - Download and verify dependencies"
	@echo "    deps-update      - Update all dependencies"
	@echo "    deps-tidy        - Tidy go.mod and go.sum"
	@echo ""
	@echo "  Testing:"
	@echo "    test             - Run unit tests"
	@echo "    test-coverage    - Run all tests with coverage report"
	@echo "    benchmark        - Run benchmarks"

# Test all packages (unit tests only, no RabbitMQ required)
test:
	@echo "📦 Running unit tests..."
	@go test -v -short -race .

# Test container package
test-coverage:
	@echo "🧪 Running container package tests..."
	@go test -v -race -coverprofile=coverage.out .
	@echo ""
	@echo "📊 Coverage Report:"
	@go tool cover -func=coverage.out

benchmark:
	@echo "🧪 Running container benchmark tests..."
	@go test -bench=. -benchmem .
	@echo ""

# Test broker package (unit tests only)
test-broker-unit:
	@echo "🧪 Running broker unit tests..."
	@go test -v -short -race ./pkg/broker/

# Test broker package (with integration)
test-broker:
	@echo "🧪 Running broker package tests (requires RabbitMQ)..."
	@go test -v -race -coverprofile=coverage_broker.out ./pkg/broker/
	@echo ""
	@echo "📊 Coverage Report:"
	@go tool cover -func=coverage_broker.out