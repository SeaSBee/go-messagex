# Makefile for go-messagex
# Production-grade messaging module for RabbitMQ with Kafka extensibility

# Variables
BINARY_NAME=go-messagex
MAIN_PATH=./cmd
BUILD_DIR=./build
COVERAGE_DIR=./coverage
VERSION?=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(shell date -u '+%Y-%m-%d_%H:%M:%S') -X main.GitCommit=$(shell git rev-parse --short HEAD)"

# Go related variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet
GOFMT=gofmt
GOLINT=golangci-lint

# Directories
PKG_DIRS=./pkg/... ./internal/...
TEST_DIRS=./tests/...
ALL_DIRS=./...

# Default target
.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo "go-messagex - Production-grade messaging module"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: all
all: clean fmt lint test build ## Run all checks and build

.PHONY: build
build: ## Build the project
	@echo "Building go-messagex..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/publisher $(MAIN_PATH)/publisher
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/consumer $(MAIN_PATH)/consumer
	@echo "Build complete. Binaries in $(BUILD_DIR)/"

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@rm -rf $(COVERAGE_DIR)
	@echo "Clean complete."

.PHONY: fmt
fmt: ## Format code with gofmt
	@echo "Formatting code..."
	$(GOFMT) -s -w $(shell find . -name "*.go" -not -path "./vendor/*")
	@echo "Formatting complete."

.PHONY: fmt-check
fmt-check: ## Check if code is properly formatted
	@echo "Checking code formatting..."
	@if [ -n "$(shell $(GOFMT) -l $(shell find . -name "*.go" -not -path "./vendor/*"))" ]; then \
		echo "Code is not formatted. Run 'make fmt' to fix."; \
		exit 1; \
	fi
	@echo "Code formatting check passed."

.PHONY: lint
lint: ## Run golangci-lint
	@echo "Running linter..."
	$(GOLINT) run $(ALL_DIRS)
	@echo "Linting complete."

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) $(ALL_DIRS)
	@echo "Go vet complete."

.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v $(ALL_DIRS)
	@echo "Tests complete."

.PHONY: test-race
test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	$(GOTEST) -race -v $(ALL_DIRS)
	@echo "Race detector tests complete."

.PHONY: test-short
test-short: ## Run short tests
	@echo "Running short tests..."
	$(GOTEST) -short -v $(ALL_DIRS)
	@echo "Short tests complete."

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -coverprofile=$(COVERAGE_DIR)/coverage.out $(ALL_DIRS)
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	$(GOCMD) tool cover -func=$(COVERAGE_DIR)/coverage.out
	@echo "Coverage report generated in $(COVERAGE_DIR)/"

.PHONY: test-bench
test-bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem $(PKG_DIRS)
	@echo "Benchmarks complete."

.PHONY: test-bench-race
test-bench-race: ## Run benchmarks with race detector
	@echo "Running benchmarks with race detector..."
	$(GOTEST) -race -bench=. -benchmem $(PKG_DIRS)
	@echo "Benchmarks with race detector complete."

.PHONY: profile-memory
profile-memory: ## Run memory profiling
	@echo "Running memory profiling..."
	$(GOTEST) -bench=. -benchmem -memprofile=memory.prof $(PKG_DIRS)
	@echo "Memory profile generated: memory.prof"
	@echo "Analyze with: go tool pprof -top memory.prof"

.PHONY: profile-cpu
profile-cpu: ## Run CPU profiling
	@echo "Running CPU profiling..."
	$(GOTEST) -bench=. -cpuprofile=cpu.prof $(PKG_DIRS)
	@echo "CPU profile generated: cpu.prof"
	@echo "Analyze with: go tool pprof -top cpu.prof"

.PHONY: profile-comprehensive
profile-comprehensive: ## Run comprehensive profiling (memory + CPU)
	@echo "Running comprehensive profiling..."
	$(GOTEST) -bench=. -benchmem -memprofile=memory.prof -cpuprofile=cpu.prof $(PKG_DIRS)
	@echo "Profiles generated: memory.prof, cpu.prof"
	@echo "Analyze memory: go tool pprof -top memory.prof"
	@echo "Analyze CPU: go tool pprof -top cpu.prof"

.PHONY: profile-benchmarks
profile-benchmarks: ## Run profiling on benchmark tests
	@echo "Running benchmark profiling..."
	$(GOTEST) -bench=. -benchmem -memprofile=memory.prof -cpuprofile=cpu.prof ./tests/benchmarks/
	@echo "Benchmark profiles generated: memory.prof, cpu.prof"

.PHONY: analyze-profiles
analyze-profiles: ## Analyze existing profile files
	@echo "Analyzing memory profile..."
	@if [ -f memory.prof ]; then \
		echo "Memory profile analysis:"; \
		go tool pprof -top memory.prof; \
	else \
		echo "No memory.prof found. Run 'make profile-memory' first."; \
	fi
	@echo ""
	@echo "Analyzing CPU profile..."
	@if [ -f cpu.prof ]; then \
		echo "CPU profile analysis:"; \
		go tool pprof -top cpu.prof; \
	else \
		echo "No cpu.prof found. Run 'make profile-cpu' first."; \
	fi

.PHONY: clean-profiles
clean-profiles: ## Clean profile files
	@echo "Cleaning profile files..."
	@rm -f *.prof
	@echo "Profile files cleaned."

.PHONY: benchmark-suite
benchmark-suite: ## Run full benchmark suite with profiling
	@echo "Running full benchmark suite..."
	@if [ -f scripts/run_benchmarks.sh ]; then \
		chmod +x scripts/run_benchmarks.sh; \
		BENCHMARK_MEMORY=true BENCHMARK_CPU=true ./scripts/run_benchmarks.sh; \
	else \
		echo "Benchmark script not found. Running basic benchmarks..."; \
		$(GOTEST) -bench=. -benchmem -memprofile=memory.prof -cpuprofile=cpu.prof ./tests/benchmarks/; \
	fi
	@echo "Benchmark suite completed."

.PHONY: verify
verify: fmt-check vet lint test-race ## Run all verification checks
	@echo "All verification checks passed!"

.PHONY: ci
ci: clean fmt-check vet lint test-race test-coverage ## Run CI pipeline locally
	@echo "CI pipeline completed successfully!"

.PHONY: deps
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	@echo "Dependencies downloaded."

.PHONY: deps-update
deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	$(GOMOD) get -u ./...
	$(GOMOD) tidy
	@echo "Dependencies updated."

.PHONY: deps-check
deps-check: ## Check for outdated dependencies
	@echo "Checking for outdated dependencies..."
	$(GOCMD) list -u -m all
	@echo "Dependency check complete."

.PHONY: install-tools
install-tools: ## Install development tools
	@echo "Installing development tools..."
	$(GOGET) -u github.com/golangci/golangci-lint/cmd/golangci-lint
	$(GOGET) -u golang.org/x/tools/cmd/goimports
	@echo "Development tools installed."

.PHONY: run-publisher
run-publisher: ## Run publisher example
	@echo "Running publisher example..."
	$(GOCMD) run $(MAIN_PATH)/publisher/main.go

.PHONY: run-consumer
run-consumer: ## Run consumer example
	@echo "Running consumer example..."
	$(GOCMD) run $(MAIN_PATH)/consumer/main.go

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .
	@echo "Docker image built: $(BINARY_NAME):$(VERSION)"

.PHONY: docker-run
docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run --rm -it $(BINARY_NAME):$(VERSION)

.PHONY: security-scan
security-scan: ## Run security scan
	@echo "Running security scan..."
	$(GOCMD) list -json -deps ./... | nancy sleuth
	@echo "Security scan complete."

.PHONY: generate
generate: ## Generate code (if needed)
	@echo "Generating code..."
	$(GOCMD) generate ./...
	@echo "Code generation complete."

.PHONY: docs
docs: ## Generate documentation
	@echo "Generating documentation..."
	$(GOCMD) doc -all ./pkg/...
	@echo "Documentation generation complete."

.PHONY: release
release: ## Create a release build
	@echo "Creating release build..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)/publisher
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)/publisher
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)/publisher
	@echo "Release builds complete in $(BUILD_DIR)/"

.PHONY: version
version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(shell git rev-parse --short HEAD)"
	@echo "Build Time: $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')"

# Development convenience targets
.PHONY: dev-setup
dev-setup: install-tools deps ## Set up development environment
	@echo "Development environment setup complete!"

.PHONY: quick-test
quick-test: test-short ## Run quick tests for development
	@echo "Quick tests complete."

.PHONY: watch
watch: ## Watch for changes and run tests (requires fswatch)
	@echo "Watching for changes..."
	@fswatch -o . | xargs -n1 -I{} make quick-test

# Cleanup targets
.PHONY: clean-all
clean-all: clean ## Clean everything including vendor
	@rm -rf vendor/
	@rm -rf .golangci-lint-cache/
	@echo "Complete cleanup finished."

.PHONY: reset
reset: clean-all deps ## Reset development environment
	@echo "Development environment reset complete."
