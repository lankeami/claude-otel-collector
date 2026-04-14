# Claude OTel Collector — Design Spec

## Overview

A custom OpenTelemetry Collector distribution that captures telemetry from Claude products (Code, Chat, Cowork), enriches it with cost/team metadata, and routes it to DataDog (metrics/traces) and S3 (full content for audit). Deployed on K8s via Helm.

## Goals

- **Cost tracking & usage analytics** — tokens, spend per model/team/project
- **Performance monitoring** — latency, error rates, throughput
- **Audit trail** — who used what, when, with configurable content capture
- **Historical preservation** — full content stored in S3 for compliance

## Architecture

```
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  Claude Code     │  │  Claude Chat     │  │  Claude Cowork   │
│  (--otel flag)   │  │  (Console API)   │  │  (Console API)   │
└───────┬─────────┘  └───────┬─────────┘  └───────┬─────────┘
        │ OTLP                │ polling             │ polling
        ▼                     ▼                     ▼
┌──────────────────────────────────────────────────────────────┐
│           Custom OTel Collector Distribution                  │
│                                                              │
│  Receivers:                                                  │
│    ├─ otlpreceiver (gRPC/HTTP) ← Claude Code traces          │
│    └─ claudereceiver (custom)  ← Console API polling          │
│                                                              │
│  Processors:                                                 │
│    ├─ claudeprocessor (custom) — enrich, normalize, cost calc│
│    └─ contentfilter (custom)   — redact/route content        │
│                                                              │
│  Exporters:                                                  │
│    ├─ datadogexporter          → DD (metrics, traces, logs)  │
│    └─ awss3exporter            → S3 (full content, audit)    │
│                                                              │
│  Deployed via Helm on K8s (standalone Deployment)            │
└──────────────────────────────────────────────────────────────┘
```

### Approach

Standard OTel Collector with custom Go processors. Built with `ocb` (OpenTelemetry Collector Builder) to bundle stock + custom components into a single binary.

### Key Decisions

- Custom collector distribution (not a sidecar pattern) — centralized collector
- Two K8s Deployments: stateless OTLP receiver (scales horizontally) and stateful API poller (single instance)
- Fan-out pipelines: same receivers feed both DD and S3 pipelines with different content filter modes

## Data Model

### Trace/Span Attributes

| Attribute | Description |
|---|---|
| `claude.product` | `code`, `chat`, `cowork` |
| `claude.model` | Model ID (e.g., `claude-sonnet-4-20250514`) |
| `claude.conversation_id` | Unique conversation identifier |
| `claude.user` | User/team identifier |
| `claude.tokens.input` | Input token count |
| `claude.tokens.output` | Output token count |
| `claude.cost.usd` | Calculated cost based on model pricing |
| `claude.tools` | Tools used in the interaction |
| `claude.error` | Error type if request failed |

### Content Attributes (configurable)

| Attribute | Description |
|---|---|
| `claude.content.prompt` | Full prompt/input |
| `claude.content.response` | Full response/output |

Content attributes are stripped by the content filter processor before hitting DataDog, and preserved in the S3 export path.

### Derived Metrics

| Metric | Type | Description |
|---|---|---|
| `claude.request.duration` | Histogram | Request latency |
| `claude.request.count` | Counter | Requests by product/model/user |
| `claude.tokens.total` | Counter | Tokens by product/model |
| `claude.cost.total` | Counter | Cost by product/model/user |
| `claude.error.count` | Counter | Errors by type |

### Pipeline Fan-Out

```
DD pipeline:  receiver → claudeprocessor → contentfilter(strip) → datadogexporter
S3 pipeline:  receiver → claudeprocessor → contentfilter(keep)  → awss3exporter
```

## Custom Components

### `claudereceiver` (V2)

Polls the Anthropic Console/Admin API for Chat and Cowork usage data.

```yaml
receivers:
  claude:
    poll_interval: 60s
    api_key: ${ANTHROPIC_API_KEY}
    products:
      - chat
      - cowork
    checkpoint_storage: /var/otel/checkpoints
```

- Converts API responses into OTel spans/metrics using the common schema
- Maintains a checkpoint to avoid re-ingesting data across restarts (PVC in K8s)
- Scope depends on what the Anthropic Console API exposes; starts simple and grows

### `claudeprocessor`

Enrichment and cost calculation in Go.

```yaml
processors:
  claude:
    pricing:
      claude-sonnet-4-20250514:
        input_per_mtok: 3.00
        output_per_mtok: 15.00
      claude-opus-4-0-20250115:
        input_per_mtok: 15.00
        output_per_mtok: 75.00
    team_mapping:
      user-123: platform-team
      user-456: ml-team
```

- **Cost calculation:** Model ID + token counts → USD cost via configurable pricing table
- **Normalization:** Ensures all spans conform to the common schema
- **Team/project tagging:** Optional user → team/project mapping for DD dashboard grouping

### `contentfilter`

Simple processor with two modes per pipeline.

```yaml
processors:
  contentfilter/dd:
    mode: strip
  contentfilter/s3:
    mode: keep
```

- **strip:** Removes `claude.content.*` attributes
- **keep:** Passes content through

## K8s Deployment

```
┌─ K8s Namespace: claude-otel ─────────────────────────┐
│                                                       │
│  Deployment: claude-otel-collector (2+ replicas)      │
│    ├─ Service: claude-otel-collector (gRPC/HTTP)      │
│    ├─ receives OTLP from Claude Code instances        │
│    └─ exports to DD + S3                              │
│                                                       │
│  Deployment: claude-otel-poller (1 replica, V2)       │
│    ├─ runs claudereceiver only                        │
│    ├─ polls Console API for Chat/Cowork               │
│    ├─ forwards to collector via OTLP                  │
│    └─ PVC for checkpoint storage                      │
│                                                       │
│  ConfigMap: collector-config                          │
│  Secret: anthropic-api-key, dd-api-key                │
└───────────────────────────────────────────────────────┘
```

### Helm Chart Values

- Replica count for collector
- API keys via Secrets (Anthropic, DataDog, AWS)
- Pricing table
- Content capture toggle
- Poll interval (V2)
- S3 bucket/prefix configuration (V2)

### Secrets

| Secret | Purpose |
|---|---|
| `ANTHROPIC_API_KEY` | Console API polling (V2) |
| `DD_API_KEY` | DataDog exporter |
| `AWS_*` | S3 exporter (or IRSA/workload identity) |

### Health Checks

Standard K8s liveness/readiness probes via the collector's built-in `healthcheck` extension.

## Phasing

### V1 — DataDog + Claude Code

- OTLP receiver for Claude Code `--otel` traces
- `claudeprocessor` for enrichment and cost calculation
- `contentfilter` in strip mode only
- DataDog exporter
- Single Deployment on K8s (no poller)
- Helm chart with basic values
- Built with `ocb`

### V2 — Add S3 + Chat/Cowork

- `claudereceiver` for Console API polling
- Second Deployment (poller) with checkpoint PVC
- `contentfilter` with keep mode for S3 pipeline
- `awss3exporter` with configurable bucket/prefix/partitioning
- S3 partitioning: `s3://<bucket>/claude-telemetry/product=<code|chat|cowork>/year=YYYY/month=MM/day=DD/`
- Content capture toggle via ConfigMap (no redeploy needed)

## Project Structure

```
claude-otel-collector/
├── builder-config.yaml          # ocb manifest
├── cmd/
│   └── collector/
│       └── main.go              # entry point (generated by ocb)
├── internal/
│   ├── claudeprocessor/         # enrichment + cost calc
│   │   ├── factory.go
│   │   ├── processor.go
│   │   ├── config.go
│   │   └── processor_test.go
│   ├── contentfilter/           # content strip/keep
│   │   ├── factory.go
│   │   ├── processor.go
│   │   ├── config.go
│   │   └── processor_test.go
│   └── claudereceiver/          # V2: Console API poller
│       ├── factory.go
│       ├── receiver.go
│       ├── config.go
│       └── receiver_test.go
├── config/
│   ├── collector-config.yaml    # OTel Collector config
│   └── poller-config.yaml       # V2: poller config
├── deploy/
│   └── helm/
│       └── claude-otel-collector/
│           ├── Chart.yaml
│           ├── values.yaml
│           └── templates/
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
└── docs/
```
