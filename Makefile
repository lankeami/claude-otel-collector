.PHONY: help install-ocb generate build test run docker-build docker-run clean

OCB_VERSION := 0.115.0
GOPATH := $(shell go env GOPATH)
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
	go test ./internal/claudeprocessor/... ./internal/contentfilter/... -v -race

run: build ## Run the collector with local config
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi && \
	./bin/claude-otel-collector --config config/collector-config.yaml

docker-build: ## Build Docker image
	docker build -t claude-otel-collector .

docker-run: docker-build ## Run collector in Docker with .env file (Datadog export)
	docker run --rm --env-file .env -p 4317:4317 -p 4318:4318 -p 13133:13133 claude-otel-collector

clean: ## Remove build artifacts
	rm -rf bin/ cmd/collector/
