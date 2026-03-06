set shell := ["bash", "-uc"]

# Default recipe - show available commands
default:
    @just --list

# Format all code using treefmt
fmt:
    treefmt --allow-missing-formatter

# Check if code is formatted correctly
check-formatted:
    treefmt --allow-missing-formatter --fail-on-change

# Run linters (from backend/)
lint:
    cd backend && golangci-lint run --timeout=2m ./...

# Run linters with auto-fix
lint-fix:
    cd backend && golangci-lint run --fix --timeout=2m ./...

# Ensure go.mod is tidy
check-tidy:
    cd backend && go mod tidy
    git diff --exit-code backend/go.mod backend/go.sum

# Run all tests
test:
    cd backend && go test ./...

# Run tests with race detector
test-race:
    cd backend && go test -race ./...

# Run tests with coverage
test-coverage:
    cd backend && go test -v -coverprofile=coverage.out ./...
    cd backend && go tool cover -html=coverage.out -o coverage.html

# Update golden test snapshots
update-golden:
    cd backend && UPDATE_GOLDEN=1 go test ./...

# Build the CLI
build:
    cd backend && go build -o ../bin/noise ./cmd/noise

# Run all checks (formatting, linting, tests, tidiness)
ci: check-formatted test lint check-tidy

# Clean build artifacts
clean:
    rm -rf bin/ backend/coverage.out backend/coverage.html
