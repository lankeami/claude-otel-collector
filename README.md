# Claude OTel Collector

Custom OpenTelemetry Collector distribution for processing Claude API telemetry data. Enriches traces with cost calculations and team attribution, then exports to DataDog.

## Architecture

```
Claude API ──OTLP──▶ [OTel Collector]
                         │
                    ┌────┴────┐
                    ▼         ▼
              batch      claudeprocessor (cost + team enrichment)
                    │         │
                    ▼         ▼
              contentfilter/dd (strip prompts/responses)
                    │
              ┌─────┴─────┐
              ▼           ▼
          DataDog       Debug
        (traces +     (local dev)
         metrics)
```

**Custom processors:**

| Processor | Purpose |
|-----------|---------|
| `claudeprocessor` | Calculates USD cost from token counts + model pricing; maps users to teams |
| `contentfilter` | Strips `claude.content.*` attributes (prompts/responses) before sending to DataDog |

## Prerequisites

- **Go 1.22+** — [install](https://go.dev/doc/install)
- **Make** — included on macOS/Linux
- **Docker** (optional) — for container builds

## Quick Start (Local)

### 1. Clone the repo

```bash
git clone https://github.com/jaychinthrajah/claude-otel-collector.git
cd claude-otel-collector
```

### 2. Install the OpenTelemetry Collector Builder

This is a one-time setup. The builder (`ocb`) generates the collector source code from `builder-config.yaml`.

```bash
make install-ocb
```

This installs `ocb` v0.115.0 to your `$GOPATH/bin`. Make sure `$GOPATH/bin` is in your `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### 3. Run the tests

Verify everything compiles and the processors work correctly:

```bash
make test
```

This runs `go test ./internal/... -v -race` across both processor packages.

### 4. Build the collector binary

```bash
make build
```

This does two things:
1. Generates the collector source into `cmd/collector/` using `ocb`
2. Compiles the binary to `bin/claude-otel-collector`

### 5. Run with the test config (no DataDog key required)

The test config uses only the `debug` exporter, so you can run locally without any external services:

```bash
./bin/claude-otel-collector --config config/test-config.yaml
```

The collector is now listening on:
- **gRPC:** `localhost:4317`
- **HTTP:** `localhost:4318`
- **Health check:** `localhost:13133`

### 6. Send a test trace

In another terminal, send a test span using `curl` against the OTLP HTTP endpoint:

```bash
curl -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{
    "resourceSpans": [{
      "scopeSpans": [{
        "spans": [{
          "traceId": "00000000000000000000000000000001",
          "spanId": "0000000000000001",
          "name": "claude-api-call",
          "kind": 1,
          "startTimeUnixNano": "1700000000000000000",
          "endTimeUnixNano": "1700000001000000000",
          "attributes": [
            {"key": "claude.model", "value": {"stringValue": "claude-sonnet-4-20250514"}},
            {"key": "claude.tokens.input", "value": {"intValue": "1000"}},
            {"key": "claude.tokens.output", "value": {"intValue": "500"}},
            {"key": "claude.user", "value": {"stringValue": "user-123"}},
            {"key": "claude.content.prompt", "value": {"stringValue": "Hello world"}}
          ]
        }]
      }]
    }]
  }'
```

Check the collector terminal — you should see the debug exporter output showing:
- `claude.cost.usd` calculated as `(1000 * 3.00 / 1,000,000) + (500 * 15.00 / 1,000,000) = 0.0105`
- `claude.team` set to `platform-team` (from the test config's team mapping)
- `claude.content.prompt` stripped by the contentfilter

### 7. Run with the production config (requires DataDog)

```bash
export DD_API_KEY="your-datadog-api-key"
make run
```

## Docker

Build and run as a container:

```bash
# Build
docker build -t claude-otel-collector .

# Run with test config (debug exporter only)
docker run -p 4317:4317 -p 4318:4318 -p 13133:13133 \
  claude-otel-collector --config /etc/otel/config.yaml

# Run with DataDog
docker run -p 4317:4317 -p 4318:4318 -p 13133:13133 \
  -e DD_API_KEY="your-key" \
  claude-otel-collector
```

## Kubernetes (Helm)

```bash
cd deploy/helm

# Install with DataDog
helm install claude-otel claude-otel-collector/ \
  --set datadogApiKey="your-key"

# Override pricing or team mappings
helm install claude-otel claude-otel-collector/ \
  --set datadogApiKey="your-key" \
  --set 'collectorConfig.teamMapping.user-123=platform-team'
```

Default Helm values: 2 replicas, 250m-1000m CPU, 256Mi-512Mi memory.

## Configuration

### Pricing

The `claudeprocessor` calculates cost using a pricing table in the config. Each model maps to input/output rates per million tokens:

```yaml
processors:
  claude:
    pricing:
      claude-sonnet-4-20250514:
        input_per_mtok: 3.00
        output_per_mtok: 15.00
```

**Cost formula:** `(input_tokens * input_per_mtok / 1,000,000) + (output_tokens * output_per_mtok / 1,000,000)`

### Team Mapping

Map user IDs to team names for attribution:

```yaml
processors:
  claude:
    team_mapping:
      user-123: platform-team
      user-456: ml-team
```

### Content Filtering

The `contentfilter` processor has two modes:

| Mode | Behavior | Use case |
|------|----------|----------|
| `strip` | Removes all `claude.content.*` attributes | DataDog pipeline — don't send prompts/responses |
| `keep` | Passes all attributes through unchanged | S3 pipeline (V2) — preserve raw content |

## Trace Attributes

All custom attributes use the `claude.` prefix:

| Attribute | Type | Source | Description |
|-----------|------|--------|-------------|
| `claude.model` | string | Instrumentation | Model name (e.g., `claude-sonnet-4-20250514`) |
| `claude.tokens.input` | int | Instrumentation | Input token count |
| `claude.tokens.output` | int | Instrumentation | Output token count |
| `claude.user` | string | Instrumentation | User identifier |
| `claude.cost.usd` | double | claudeprocessor | Calculated cost in USD |
| `claude.team` | string | claudeprocessor | Team attribution from mapping |
| `claude.content.*` | string | Instrumentation | Prompt/response content (stripped by contentfilter) |

## Project Structure

```
├── builder-config.yaml              # OTel Collector Builder manifest
├── Makefile                         # Build, test, run targets
├── Dockerfile                       # Multi-stage container build
├── config/
│   ├── collector-config.yaml        # Production config (DataDog + debug)
│   └── test-config.yaml             # Local testing (debug exporter only)
├── internal/
│   ├── claudeprocessor/             # Cost calculation + team mapping
│   │   ├── config.go
│   │   ├── factory.go
│   │   ├── processor.go
│   │   └── *_test.go
│   └── contentfilter/               # Strip/keep content attributes
│       ├── config.go
│       ├── factory.go
│       ├── processor.go
│       └── *_test.go
├── cmd/collector/                   # Generated by ocb (do not edit)
├── deploy/helm/claude-otel-collector/  # Kubernetes Helm chart
└── docs/superpowers/                # Design specs and implementation plans
```

## Make Targets

```
$ make help
build                Build the collector binary
clean                Remove build artifacts
generate             Generate collector source from builder-config.yaml
help                 Show this help
install-ocb          Install OpenTelemetry Collector Builder
run                  Run the collector with local config
test                 Run all tests
```

## Adding a New Processor

1. Create a new package under `internal/` with four files:
   - `config.go` — Config struct with `mapstructure` tags and `Validate()` method
   - `factory.go` — `NewFactory()` registering the component type
   - `processor.go` — `ConsumeTraces()` implementation
   - `processor_test.go` — Tests using `consumertest.TracesSink` and testify
2. Add to `builder-config.yaml` under `processors`
3. Add to the pipeline in `config/collector-config.yaml`
4. Run `make build` to regenerate the collector
