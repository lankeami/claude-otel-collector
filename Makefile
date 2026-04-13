.PHONY: help install-ocb generate build test run clean

OCB_VERSION := 0.115.0
OCB := $(shell which ocb 2>/dev/null || echo $(GOPATH)/bin/ocb)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

install-ocb: ## Install OpenTelemetry Collector Builder
	go install go.opentelemetry.io/collector/cmd/builder@v$(OCB_VERSION)
	@mv $(GOPATH)/bin/builder $(GOPATH)/bin/ocb 2>/dev/null || true

generate: ## Generate collector source from builder-config.yaml (source only, no compile)
	$(OCB) --skip-compilation --config builder-config.yaml
	@# Fix module name so Go's internal package rule allows importing ./internal/*
	@sed -i '' 's|^module go.opentelemetry.io/collector/cmd/builder|module github.com/jaychinthrajah/claude-otel-collector/cmd/collector|' cmd/collector/go.mod

build: generate ## Build the collector binary
	mkdir -p bin
	cd cmd/collector && go build -o ../../bin/claude-otel-collector .

test: ## Run all tests
	go test ./internal/... -v -race

run: build ## Run the collector with local config
	./bin/claude-otel-collector --config config/collector-config.yaml

clean: ## Remove build artifacts
	rm -rf bin/ cmd/collector/
