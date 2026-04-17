# Claude OTel Collector

Custom OpenTelemetry Collector distribution for processing Claude API telemetry data. Enriches traces with cost calculations and team attribution, then routes to DataDog (metrics/traces) and optionally S3 (raw content).

## Build & Run

```bash
make install-ocb  # Install OTel Collector Builder v0.115.0 (one-time)
make generate      # Generate collector source from builder-config.yaml
make build         # Generate + compile binary to bin/claude-otel-collector
make run           # Build + run with config/collector-config.yaml
make clean         # Remove bin/ and generated cmd/collector/
```

**Docker:**
```bash
docker build -t claude-otel-collector .
```

## Testing

```bash
make test          # Run all tests with race detection
go test ./internal/... -v -race  # Direct equivalent
```

Tests live alongside source in each processor package. No integration tests yet — all unit tests using `consumertest.TracesSink` as mock consumers.

## Project Structure

```
builder-config.yaml          # OCB manifest — defines collector components
cmd/collector/               # Generated collector entry point (do not edit)
config/
  collector-config.yaml      # Production pipeline config
  test-config.yaml           # Local testing config (debug exporter only)
internal/
  claudeprocessor/           # Cost calculation + team mapping processor
  contentfilter/             # Strip/keep content attributes processor
deploy/helm/claude-otel-collector/  # Kubernetes Helm chart
docs/superpowers/            # Design specs and implementation plans
```

## Key Patterns

### OTel Processor Structure

Each custom processor follows the standard OTel component pattern with four files:

- `config.go` — Config struct with `mapstructure` tags + `Validate()` method
- `factory.go` — `NewFactory()` registering the processor type and default config
- `processor.go` — `ConsumeTraces()` implementation iterating ResourceSpans > ScopeSpans > Spans
- `*_test.go` — Unit tests building traces with `ptrace.NewTraces()` and asserting via testify

### Trace Attributes

All custom attributes use the `claude.` prefix:
- `claude.model` — Model name (e.g., "claude-sonnet-4-20250514")
- `claude.tokens.input`, `claude.tokens.output` — Token counts
- `claude.cost.usd` — Calculated cost (added by claudeprocessor)
- `claude.team` — Team attribution (added by claudeprocessor)
- `claude.content.*` — Prompt/response content (stripped by contentfilter in DD pipeline)

### Adding a New Processor

1. Create package under `internal/`
2. Implement config, factory, processor, and tests following existing patterns
3. Add to `builder-config.yaml` under `processors`
4. Add to `config/collector-config.yaml` pipeline
5. Run `make build` to regenerate

## Dependencies

- **Go 1.22+** (go.mod says 1.26.1 but builder uses 1.22)
- **OpenTelemetry Collector v0.115.0** — core framework
- **OTel Collector Builder (ocb) v0.115.0** — generates collector from manifest
- **testify** — test assertions
- **zap** — structured logging

## Deployment

Helm chart at `deploy/helm/claude-otel-collector/`:
- 2 replicas default, ClusterIP service
- Ports: 4317 (gRPC), 4318 (HTTP), 13133 (health check at `/ping`)
- Requires `DD_API_KEY` secret
- Pricing and team mappings configured via Helm values

## Current Status

**V1 complete:** claudeprocessor, contentfilter, DataDog export, Helm chart, Docker build.
**V2 planned:** claudereceiver (API polling), S3 exporter, chat/cowork session support. See `docs/superpowers/specs/` for design.

## Telemetry Metrics Catalog

See [`docs/superpowers/specs/2026-04-17-telemetry-metrics-catalog.md`](docs/superpowers/specs/2026-04-17-telemetry-metrics-catalog.md) for a comprehensive catalog of every metric, attribute, and data point available from Claude Code, Claude Chat, Claude Cowork, and the Anthropic Admin APIs — organized by privacy tier (Default → Tool Details → User Prompts → Tool Content → Raw API Bodies) with compliance mapping for SOC 2, GDPR/CCPA, and internal policy.
