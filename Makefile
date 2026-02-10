.PHONY: build test clean install fmt lint

BINARY_NAME=costctl
BUILD_DIR=.
INSTALL_PATH=$(HOME)/.local/bin

# Build the binary
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -cover ./...

# Clean build artifacts
clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)

# Install locally
install: build
	mkdir -p $(INSTALL_PATH)
	cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_PATH)/
	@echo "Installed to $(INSTALL_PATH)/$(BINARY_NAME)"

# Format code
fmt:
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Run the tool with sample data
run-today:
	go run . report --period today

run-week:
	go run . report --period week

run-full:
	go run . report --full

run-crons:
	go run . report --crons

run-models:
	go run . report --models

# Development helpers
dev-build:
	go build -o $(BINARY_NAME) .
	./$(BINARY_NAME) agents

.DEFAULT_GOAL := build
