# Claude OTel Collector V1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a custom OTel Collector distribution that receives Claude Code OTLP traces, enriches them with cost/team metadata, strips content attributes, and exports to DataDog.

**Architecture:** Custom OTel Collector built with `ocb`. OTLP receiver accepts Claude Code `--otel` traces. A custom `claudeprocessor` enriches spans with cost calculations and team tags. A `contentfilter` processor strips content attributes. The DataDog exporter sends metrics and traces to DD. Deployed as a single K8s Deployment via Helm.

**Tech Stack:** Go 1.22+, OpenTelemetry Collector v0.115+, `ocb` (OTel Collector Builder), DataDog exporter, Docker, Helm 3, Kubernetes

---

## File Structure

```
claude-otel-collector/
├── builder-config.yaml                    # ocb manifest — declares all components
├── go.mod                                 # Go module definition
├── go.sum                                 # Go dependency checksums
├── Makefile                               # Build, test, run targets
├── Dockerfile                             # Multi-stage build for collector binary
├── internal/
│   ├── claudeprocessor/
│   │   ├── config.go                      # Config struct + validation
│   │   ├── config_test.go                 # Config validation tests
│   │   ├── factory.go                     # OTel component factory
│   │   ├── processor.go                   # Enrichment + cost calc logic
│   │   └── processor_test.go              # Processor unit tests
│   └── contentfilter/
│       ├── config.go                      # Config struct (strip/keep mode)
│       ├── config_test.go                 # Config validation tests
│       ├── factory.go                     # OTel component factory
│       ├── processor.go                   # Attribute strip/keep logic
│       └── processor_test.go              # Processor unit tests
├── config/
│   └── collector-config.yaml              # OTel Collector pipeline config
├── deploy/
│   └── helm/
│       └── claude-otel-collector/
│           ├── Chart.yaml                 # Helm chart metadata
│           ├── values.yaml                # Default values
│           └── templates/
│               ├── deployment.yaml        # Collector Deployment
│               ├── service.yaml           # OTLP Service (gRPC + HTTP)
│               ├── configmap.yaml         # Collector config
│               └── secret.yaml            # DD API key
└── docs/
```

---

### Task 1: Project Scaffolding + Go Module

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `builder-config.yaml`

This task sets up the Go module and `ocb` builder config. The `ocb` tool reads `builder-config.yaml` to generate a custom collector binary that includes our custom components plus stock components (OTLP receiver, DataDog exporter, healthcheck extension).

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd /Users/jaychinthrajah/workspaces/claude-otel-collector
go mod init github.com/jaychinthrajah/claude-otel-collector
```

Expected: `go.mod` file created with module path.

- [ ] **Step 2: Create builder-config.yaml**

This is the manifest that tells `ocb` which components to include in the collector binary. It references our custom processors (which we'll build in later tasks) and stock components from the contrib repo.

Create `builder-config.yaml`:
```yaml
dist:
  name: claude-otel-collector
  description: Custom OTel Collector for Claude telemetry
  output_path: ./cmd/collector
  otelcol_version: "0.115.0"

extensions:
  - gomod: go.opentelemetry.io/collector/extension/zpagesextension v0.115.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.115.0

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/otlpreceiver v0.115.0

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.115.0
  - gomod: github.com/jaychinthrajah/claude-otel-collector/internal/claudeprocessor v0.0.0
    path: ./internal/claudeprocessor
  - gomod: github.com/jaychinthrajah/claude-otel-collector/internal/contentfilter v0.0.0
    path: ./internal/contentfilter

exporters:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter v0.115.0
  - gomod: go.opentelemetry.io/collector/exporter/debugexporter v0.115.0
```

- [ ] **Step 3: Create Makefile**

Create `Makefile`:
```makefile
.PHONY: help install-ocb generate build test run clean

OCB_VERSION := 0.115.0
OCB := $(shell which ocb 2>/dev/null || echo $(GOPATH)/bin/ocb)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

install-ocb: ## Install OpenTelemetry Collector Builder
	go install go.opentelemetry.io/collector/cmd/builder@v$(OCB_VERSION)
	@mv $(GOPATH)/bin/builder $(GOPATH)/bin/ocb 2>/dev/null || true

generate: ## Generate collector source from builder-config.yaml
	$(OCB) --config builder-config.yaml

build: generate ## Build the collector binary
	cd cmd/collector && go build -o ../../bin/claude-otel-collector .

test: ## Run all tests
	go test ./internal/... -v -race

run: build ## Run the collector with local config
	./bin/claude-otel-collector --config config/collector-config.yaml

clean: ## Remove build artifacts
	rm -rf bin/ cmd/collector/
```

- [ ] **Step 4: Commit**

```bash
git init
git add go.mod builder-config.yaml Makefile
git commit -m "feat: scaffold project with go module, ocb config, and Makefile"
```

---

### Task 2: claudeprocessor — Config

**Files:**
- Create: `internal/claudeprocessor/config.go`
- Create: `internal/claudeprocessor/config_test.go`
- Create: `internal/claudeprocessor/go.mod`

The `claudeprocessor` config defines the pricing table and team mapping. Each OTel component in a custom distribution needs its own `go.mod` so `ocb` can reference it.

- [ ] **Step 1: Create component go.mod**

Create `internal/claudeprocessor/go.mod`:
```
module github.com/jaychinthrajah/claude-otel-collector/internal/claudeprocessor

go 1.22.0
```

Then run:
```bash
cd internal/claudeprocessor && go get go.opentelemetry.io/collector/component@v0.115.0 && go get go.opentelemetry.io/collector/processor@v0.115.0 && cd ../..
```

- [ ] **Step 2: Write config test**

Create `internal/claudeprocessor/config_test.go`:
```go
package claudeprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := &Config{
		Pricing: map[string]ModelPricing{
			"claude-sonnet-4-20250514": {
				InputPerMTok:  3.00,
				OutputPerMTok: 15.00,
			},
		},
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestConfig_Validate_EmptyPricing(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pricing")
}

func TestConfig_Validate_NegativePrice(t *testing.T) {
	cfg := &Config{
		Pricing: map[string]ModelPricing{
			"claude-sonnet-4-20250514": {
				InputPerMTok:  -1.0,
				OutputPerMTok: 15.00,
			},
		},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "negative")
}

func TestConfig_Validate_WithTeamMapping(t *testing.T) {
	cfg := &Config{
		Pricing: map[string]ModelPricing{
			"claude-sonnet-4-20250514": {
				InputPerMTok:  3.00,
				OutputPerMTok: 15.00,
			},
		},
		TeamMapping: map[string]string{
			"user-123": "platform-team",
		},
	}
	err := cfg.Validate()
	require.NoError(t, err)
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd internal/claudeprocessor && go test -v -run TestConfig
```

Expected: Compilation error — `Config`, `ModelPricing` not defined.

- [ ] **Step 4: Implement config**

Create `internal/claudeprocessor/config.go`:
```go
package claudeprocessor

import (
	"errors"
	"fmt"
)

// Config holds configuration for the Claude processor.
type Config struct {
	// Pricing maps model IDs to their per-million-token pricing.
	Pricing map[string]ModelPricing `mapstructure:"pricing"`
	// TeamMapping maps user identifiers to team names for DD grouping.
	TeamMapping map[string]string `mapstructure:"team_mapping"`
}

// ModelPricing defines per-million-token costs for a model.
type ModelPricing struct {
	InputPerMTok  float64 `mapstructure:"input_per_mtok"`
	OutputPerMTok float64 `mapstructure:"output_per_mtok"`
}

func (cfg *Config) Validate() error {
	if len(cfg.Pricing) == 0 {
		return errors.New("pricing table must not be empty")
	}
	for model, pricing := range cfg.Pricing {
		if pricing.InputPerMTok < 0 {
			return fmt.Errorf("model %q: input_per_mtok must not be negative", model)
		}
		if pricing.OutputPerMTok < 0 {
			return fmt.Errorf("model %q: output_per_mtok must not be negative", model)
		}
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd internal/claudeprocessor && go test -v -run TestConfig
```

Expected: All 4 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/claudeprocessor/go.mod internal/claudeprocessor/go.sum internal/claudeprocessor/config.go internal/claudeprocessor/config_test.go
git commit -m "feat(claudeprocessor): add config with pricing table and team mapping"
```

---

### Task 3: claudeprocessor — Factory

**Files:**
- Create: `internal/claudeprocessor/factory.go`

The factory registers the processor component with the OTel Collector framework. It defines the component type name, default config, and constructor functions.

- [ ] **Step 1: Implement factory**

Create `internal/claudeprocessor/factory.go`:
```go
package claudeprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	typeStr   = "claude"
	stability = component.StabilityLevelDevelopment
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
		processor.WithMetrics(createMetricsProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Pricing:     make(map[string]ModelPricing),
		TeamMapping: make(map[string]string),
	}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	pCfg := cfg.(*Config)
	return newClaudeProcessor(set.Logger, pCfg, nextConsumer), nil
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	// Metrics pass through — enrichment happens on traces
	return newClaudeMetricsProcessor(set.Logger, pCfg, nextConsumer), nil
}
```

Note: `newClaudeProcessor` and `newClaudeMetricsProcessor` are implemented in Task 4. This file will not compile until then.

- [ ] **Step 2: Commit**

```bash
git add internal/claudeprocessor/factory.go
git commit -m "feat(claudeprocessor): add OTel component factory"
```

---

### Task 4: claudeprocessor — Trace Processor Logic

**Files:**
- Create: `internal/claudeprocessor/processor.go`
- Create: `internal/claudeprocessor/processor_test.go`

This is the core business logic. It reads spans, calculates cost from token counts + pricing table, applies team mapping, and sets `claude.cost.usd` and `claude.team` attributes.

- [ ] **Step 1: Write processor tests**

Create `internal/claudeprocessor/processor_test.go`:
```go
package claudeprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func newTestTraces(model string, inputTokens, outputTokens int64, user string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("claude.request")
	attrs := span.Attributes()
	attrs.PutStr("claude.model", model)
	attrs.PutInt("claude.tokens.input", inputTokens)
	attrs.PutInt("claude.tokens.output", outputTokens)
	if user != "" {
		attrs.PutStr("claude.user", user)
	}
	return td
}

func TestProcessor_CostCalculation(t *testing.T) {
	cfg := &Config{
		Pricing: map[string]ModelPricing{
			"claude-sonnet-4-20250514": {
				InputPerMTok:  3.00,
				OutputPerMTok: 15.00,
			},
		},
	}
	sink := &consumertest.TracesSink{}
	proc := newClaudeProcessor(zap.NewNop(), cfg, sink)

	td := newTestTraces("claude-sonnet-4-20250514", 1000, 500, "")
	err := proc.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	require.Equal(t, 1, sink.SpanCount())
	spans := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	costVal, ok := spans.At(0).Attributes().Get("claude.cost.usd")
	require.True(t, ok)
	// 1000 input tokens * 3.00/1M + 500 output tokens * 15.00/1M = 0.003 + 0.0075 = 0.0105
	assert.InDelta(t, 0.0105, costVal.Double(), 0.0001)
}

func TestProcessor_UnknownModel(t *testing.T) {
	cfg := &Config{
		Pricing: map[string]ModelPricing{
			"claude-sonnet-4-20250514": {
				InputPerMTok:  3.00,
				OutputPerMTok: 15.00,
			},
		},
	}
	sink := &consumertest.TracesSink{}
	proc := newClaudeProcessor(zap.NewNop(), cfg, sink)

	td := newTestTraces("unknown-model", 1000, 500, "")
	err := proc.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	require.Equal(t, 1, sink.SpanCount())
	spans := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	_, ok := spans.At(0).Attributes().Get("claude.cost.usd")
	assert.False(t, ok, "should not set cost for unknown model")
}

func TestProcessor_TeamMapping(t *testing.T) {
	cfg := &Config{
		Pricing: map[string]ModelPricing{
			"claude-sonnet-4-20250514": {
				InputPerMTok:  3.00,
				OutputPerMTok: 15.00,
			},
		},
		TeamMapping: map[string]string{
			"user-123": "platform-team",
		},
	}
	sink := &consumertest.TracesSink{}
	proc := newClaudeProcessor(zap.NewNop(), cfg, sink)

	td := newTestTraces("claude-sonnet-4-20250514", 1000, 500, "user-123")
	err := proc.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	spans := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	teamVal, ok := spans.At(0).Attributes().Get("claude.team")
	require.True(t, ok)
	assert.Equal(t, "platform-team", teamVal.Str())
}

func TestProcessor_NoTeamMapping(t *testing.T) {
	cfg := &Config{
		Pricing: map[string]ModelPricing{
			"claude-sonnet-4-20250514": {
				InputPerMTok:  3.00,
				OutputPerMTok: 15.00,
			},
		},
		TeamMapping: map[string]string{
			"user-123": "platform-team",
		},
	}
	sink := &consumertest.TracesSink{}
	proc := newClaudeProcessor(zap.NewNop(), cfg, sink)

	td := newTestTraces("claude-sonnet-4-20250514", 1000, 500, "user-999")
	err := proc.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	spans := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	_, ok := spans.At(0).Attributes().Get("claude.team")
	assert.False(t, ok, "should not set team for unmapped user")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd internal/claudeprocessor && go test -v -run TestProcessor
```

Expected: Compilation error — `newClaudeProcessor` not defined.

- [ ] **Step 3: Implement processor**

Create `internal/claudeprocessor/processor.go`:
```go
package claudeprocessor

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type claudeProcessor struct {
	logger      *zap.Logger
	cfg         *Config
	nextTraces  consumer.Traces
}

type claudeMetricsProcessor struct {
	logger      *zap.Logger
	cfg         *Config
	nextMetrics consumer.Metrics
}

func newClaudeProcessor(logger *zap.Logger, cfg *Config, next consumer.Traces) *claudeProcessor {
	return &claudeProcessor{
		logger:     logger,
		cfg:        cfg,
		nextTraces: next,
	}
}

func newClaudeMetricsProcessor(logger *zap.Logger, cfg *Config, next consumer.Metrics) *claudeMetricsProcessor {
	return &claudeMetricsProcessor{
		logger:      logger,
		cfg:         cfg,
		nextMetrics: next,
	}
}

func (p *claudeProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				p.processSpan(spans.At(k))
			}
		}
	}
	return p.nextTraces.ConsumeTraces(ctx, td)
}

func (p *claudeProcessor) processSpan(span ptrace.Span) {
	attrs := span.Attributes()

	// Cost calculation
	modelVal, ok := attrs.Get("claude.model")
	if ok {
		model := modelVal.Str()
		if pricing, found := p.cfg.Pricing[model]; found {
			inputTokens := int64(0)
			outputTokens := int64(0)

			if v, ok := attrs.Get("claude.tokens.input"); ok {
				inputTokens = v.Int()
			}
			if v, ok := attrs.Get("claude.tokens.output"); ok {
				outputTokens = v.Int()
			}

			cost := (float64(inputTokens) * pricing.InputPerMTok / 1_000_000) +
				(float64(outputTokens) * pricing.OutputPerMTok / 1_000_000)
			attrs.PutDouble("claude.cost.usd", cost)
		} else {
			p.logger.Debug("no pricing found for model", zap.String("model", model))
		}
	}

	// Team mapping
	if userVal, ok := attrs.Get("claude.user"); ok {
		user := userVal.Str()
		if team, found := p.cfg.TeamMapping[user]; found {
			attrs.PutStr("claude.team", team)
		}
	}
}

func (p *claudeProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *claudeProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (p *claudeProcessor) Shutdown(_ context.Context) error {
	return nil
}

func (p *claudeMetricsProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return p.nextMetrics.ConsumeMetrics(ctx, md)
}

func (p *claudeMetricsProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (p *claudeMetricsProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (p *claudeMetricsProcessor) Shutdown(_ context.Context) error {
	return nil
}
```

Note: Add the missing `component` import:
```go
import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)
```

- [ ] **Step 4: Fix factory.go — metrics processor reference**

In `internal/claudeprocessor/factory.go`, the `createMetricsProcessor` function has a bug — it references `pCfg` but should use `cfg`. Fix it:

```go
func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	mCfg := cfg.(*Config)
	return newClaudeMetricsProcessor(set.Logger, mCfg, nextConsumer), nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd internal/claudeprocessor && go test -v -run TestProcessor
```

Expected: All 4 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/claudeprocessor/processor.go internal/claudeprocessor/processor_test.go internal/claudeprocessor/factory.go
git commit -m "feat(claudeprocessor): implement cost calculation and team mapping"
```

---

### Task 5: contentfilter — Config

**Files:**
- Create: `internal/contentfilter/go.mod`
- Create: `internal/contentfilter/config.go`
- Create: `internal/contentfilter/config_test.go`

The contentfilter processor has a single config field: `mode` (either `strip` or `keep`).

- [ ] **Step 1: Create component go.mod**

Create `internal/contentfilter/go.mod`:
```
module github.com/jaychinthrajah/claude-otel-collector/internal/contentfilter

go 1.22.0
```

Then run:
```bash
cd internal/contentfilter && go get go.opentelemetry.io/collector/component@v0.115.0 && go get go.opentelemetry.io/collector/processor@v0.115.0 && cd ../..
```

- [ ] **Step 2: Write config test**

Create `internal/contentfilter/config_test.go`:
```go
package contentfilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_Strip(t *testing.T) {
	cfg := &Config{Mode: ModeStrip}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestConfig_Validate_Keep(t *testing.T) {
	cfg := &Config{Mode: ModeKeep}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestConfig_Validate_Invalid(t *testing.T) {
	cfg := &Config{Mode: "invalid"}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mode")
}

func TestConfig_Validate_Empty(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate()
	require.Error(t, err)
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd internal/contentfilter && go test -v -run TestConfig
```

Expected: Compilation error — `Config`, `ModeStrip`, `ModeKeep` not defined.

- [ ] **Step 4: Implement config**

Create `internal/contentfilter/config.go`:
```go
package contentfilter

import "fmt"

const (
	ModeStrip = "strip"
	ModeKeep  = "keep"
)

type Config struct {
	// Mode determines whether to strip or keep claude.content.* attributes.
	// Valid values: "strip", "keep"
	Mode string `mapstructure:"mode"`
}

func (cfg *Config) Validate() error {
	switch cfg.Mode {
	case ModeStrip, ModeKeep:
		return nil
	default:
		return fmt.Errorf("mode must be %q or %q, got %q", ModeStrip, ModeKeep, cfg.Mode)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd internal/contentfilter && go test -v -run TestConfig
```

Expected: All 4 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/contentfilter/go.mod internal/contentfilter/go.sum internal/contentfilter/config.go internal/contentfilter/config_test.go
git commit -m "feat(contentfilter): add config with strip/keep mode"
```

---

### Task 6: contentfilter — Factory + Processor

**Files:**
- Create: `internal/contentfilter/factory.go`
- Create: `internal/contentfilter/processor.go`
- Create: `internal/contentfilter/processor_test.go`

- [ ] **Step 1: Write processor tests**

Create `internal/contentfilter/processor_test.go`:
```go
package contentfilter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func newTestTraces(withContent bool) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("claude.request")
	attrs := span.Attributes()
	attrs.PutStr("claude.model", "claude-sonnet-4-20250514")
	attrs.PutStr("claude.user", "user-123")
	attrs.PutInt("claude.tokens.input", 1000)
	if withContent {
		attrs.PutStr("claude.content.prompt", "Hello, tell me about Go")
		attrs.PutStr("claude.content.response", "Go is a programming language...")
	}
	return td
}

func TestProcessor_StripMode(t *testing.T) {
	cfg := &Config{Mode: ModeStrip}
	sink := &consumertest.TracesSink{}
	proc := newContentFilterProcessor(zap.NewNop(), cfg, sink)

	td := newTestTraces(true)
	err := proc.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	require.Equal(t, 1, sink.SpanCount())
	spans := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	attrs := spans.At(0).Attributes()

	_, hasPrompt := attrs.Get("claude.content.prompt")
	_, hasResponse := attrs.Get("claude.content.response")
	assert.False(t, hasPrompt, "prompt should be stripped")
	assert.False(t, hasResponse, "response should be stripped")

	// Non-content attributes preserved
	_, hasModel := attrs.Get("claude.model")
	assert.True(t, hasModel, "model should be preserved")
}

func TestProcessor_KeepMode(t *testing.T) {
	cfg := &Config{Mode: ModeKeep}
	sink := &consumertest.TracesSink{}
	proc := newContentFilterProcessor(zap.NewNop(), cfg, sink)

	td := newTestTraces(true)
	err := proc.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	spans := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	attrs := spans.At(0).Attributes()

	promptVal, hasPrompt := attrs.Get("claude.content.prompt")
	assert.True(t, hasPrompt, "prompt should be kept")
	assert.Equal(t, "Hello, tell me about Go", promptVal.Str())
}

func TestProcessor_StripMode_NoContent(t *testing.T) {
	cfg := &Config{Mode: ModeStrip}
	sink := &consumertest.TracesSink{}
	proc := newContentFilterProcessor(zap.NewNop(), cfg, sink)

	td := newTestTraces(false)
	err := proc.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	require.Equal(t, 1, sink.SpanCount())
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd internal/contentfilter && go test -v -run TestProcessor
```

Expected: Compilation error — `newContentFilterProcessor` not defined.

- [ ] **Step 3: Implement processor**

Create `internal/contentfilter/processor.go`:
```go
package contentfilter

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

const contentAttributePrefix = "claude.content."

type contentFilterProcessor struct {
	logger *zap.Logger
	cfg    *Config
	next   consumer.Traces
}

func newContentFilterProcessor(logger *zap.Logger, cfg *Config, next consumer.Traces) *contentFilterProcessor {
	return &contentFilterProcessor{
		logger: logger,
		cfg:    cfg,
		next:   next,
	}
}

func (p *contentFilterProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if p.cfg.Mode == ModeKeep {
		return p.next.ConsumeTraces(ctx, td)
	}

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				p.stripContent(spans.At(k))
			}
		}
	}
	return p.next.ConsumeTraces(ctx, td)
}

func (p *contentFilterProcessor) stripContent(span ptrace.Span) {
	attrs := span.Attributes()
	toRemove := []string{}
	attrs.Range(func(k string, _ ptrace.Span) bool {
		if strings.HasPrefix(k, contentAttributePrefix) {
			toRemove = append(toRemove, k)
		}
		return true
	})
	for _, key := range toRemove {
		attrs.Remove(key)
	}
}

func (p *contentFilterProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: p.cfg.Mode == ModeStrip}
}

func (p *contentFilterProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (p *contentFilterProcessor) Shutdown(_ context.Context) error {
	return nil
}
```

Note: The `attrs.Range` callback signature uses `pcommon.Value`, not `ptrace.Span`. Fix:
```go
import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)
```

And the Range callback:
```go
attrs.Range(func(k string, _ pcommon.Value) bool {
    if strings.HasPrefix(k, contentAttributePrefix) {
        toRemove = append(toRemove, k)
    }
    return true
})
```

- [ ] **Step 4: Implement factory**

Create `internal/contentfilter/factory.go`:
```go
package contentfilter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	typeStr   = "contentfilter"
	stability = component.StabilityLevelDevelopment
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Mode: ModeStrip,
	}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	fCfg := cfg.(*Config)
	return newContentFilterProcessor(set.Logger, fCfg, nextConsumer), nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd internal/contentfilter && go test -v -run TestProcessor
```

Expected: All 3 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/contentfilter/factory.go internal/contentfilter/processor.go internal/contentfilter/processor_test.go
git commit -m "feat(contentfilter): implement strip/keep processor with factory"
```

---

### Task 7: Collector Config + Build

**Files:**
- Create: `config/collector-config.yaml`

This is the OTel Collector pipeline configuration that wires receivers → processors → exporters for V1.

- [ ] **Step 1: Create collector config**

Create `config/collector-config.yaml`:
```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 5s
    send_batch_size: 1000
  claude:
    pricing:
      claude-sonnet-4-20250514:
        input_per_mtok: 3.00
        output_per_mtok: 15.00
      claude-opus-4-0-20250115:
        input_per_mtok: 15.00
        output_per_mtok: 75.00
      claude-haiku-4-5-20251001:
        input_per_mtok: 0.80
        output_per_mtok: 4.00
    team_mapping: {}
  contentfilter/dd:
    mode: strip

exporters:
  datadog:
    api:
      key: ${DD_API_KEY}
      site: datadoghq.com
  debug:
    verbosity: detailed

extensions:
  health_check:
    endpoint: 0.0.0.0:13133

service:
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch, claude, contentfilter/dd]
      exporters: [datadog, debug]
    metrics:
      receivers: [otlp]
      processors: [batch, claude]
      exporters: [datadog]
```

- [ ] **Step 2: Build the collector with ocb**

```bash
make install-ocb
make generate
```

Expected: `cmd/collector/` directory generated with `main.go` and `components.go`. If there are dependency resolution issues, run:
```bash
cd cmd/collector && go mod tidy && cd ../..
```

- [ ] **Step 3: Build the binary**

```bash
make build
```

Expected: `bin/claude-otel-collector` binary created.

- [ ] **Step 4: Verify the collector starts with the config**

```bash
DD_API_KEY=test ./bin/claude-otel-collector --config config/collector-config.yaml &
sleep 2
curl -s http://localhost:13133/
kill %1
```

Expected: Health check returns OK. The collector will log errors about the invalid DD API key, which is expected — we're verifying the pipeline wiring.

- [ ] **Step 5: Commit**

```bash
git add config/collector-config.yaml cmd/ go.sum
git commit -m "feat: add collector config and generate collector binary with ocb"
```

---

### Task 8: Dockerfile

**Files:**
- Create: `Dockerfile`

Multi-stage build: build the collector in a Go image, copy the binary into a minimal runtime image.

- [ ] **Step 1: Create Dockerfile**

Create `Dockerfile`:
```dockerfile
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache make git

# Install ocb
RUN go install go.opentelemetry.io/collector/cmd/builder@v0.115.0 \
    && mv /go/bin/builder /go/bin/ocb

WORKDIR /src
COPY . .

RUN ocb --config builder-config.yaml
RUN cd cmd/collector && go build -o /claude-otel-collector .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /claude-otel-collector /claude-otel-collector
COPY config/collector-config.yaml /etc/otel/config.yaml

EXPOSE 4317 4318 13133

ENTRYPOINT ["/claude-otel-collector"]
CMD ["--config", "/etc/otel/config.yaml"]
```

- [ ] **Step 2: Build the Docker image**

```bash
docker build -t claude-otel-collector:dev .
```

Expected: Image builds successfully.

- [ ] **Step 3: Smoke test the container**

```bash
docker run --rm -d --name otel-test -e DD_API_KEY=test -p 13133:13133 claude-otel-collector:dev
sleep 3
curl -s http://localhost:13133/
docker stop otel-test
```

Expected: Health check returns OK.

- [ ] **Step 4: Commit**

```bash
git add Dockerfile
git commit -m "feat: add multi-stage Dockerfile for collector"
```

---

### Task 9: Helm Chart

**Files:**
- Create: `deploy/helm/claude-otel-collector/Chart.yaml`
- Create: `deploy/helm/claude-otel-collector/values.yaml`
- Create: `deploy/helm/claude-otel-collector/templates/deployment.yaml`
- Create: `deploy/helm/claude-otel-collector/templates/service.yaml`
- Create: `deploy/helm/claude-otel-collector/templates/configmap.yaml`
- Create: `deploy/helm/claude-otel-collector/templates/secret.yaml`

- [ ] **Step 1: Create Chart.yaml**

Create `deploy/helm/claude-otel-collector/Chart.yaml`:
```yaml
apiVersion: v2
name: claude-otel-collector
description: Custom OTel Collector for Claude telemetry
type: application
version: 0.1.0
appVersion: "0.1.0"
```

- [ ] **Step 2: Create values.yaml**

Create `deploy/helm/claude-otel-collector/values.yaml`:
```yaml
replicaCount: 2

image:
  repository: claude-otel-collector
  tag: "0.1.0"
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  grpcPort: 4317
  httpPort: 4318

resources:
  requests:
    cpu: 250m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 512Mi

datadogApiKey: ""

collectorConfig:
  pricing:
    claude-sonnet-4-20250514:
      input_per_mtok: 3.00
      output_per_mtok: 15.00
    claude-opus-4-0-20250115:
      input_per_mtok: 15.00
      output_per_mtok: 75.00
    claude-haiku-4-5-20251001:
      input_per_mtok: 0.80
      output_per_mtok: 4.00
  teamMapping: {}
  datadogSite: datadoghq.com
```

- [ ] **Step 3: Create configmap.yaml**

Create `deploy/helm/claude-otel-collector/templates/configmap.yaml`:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-config
  labels:
    app: {{ .Release.Name }}
data:
  collector-config.yaml: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318

    processors:
      batch:
        timeout: 5s
        send_batch_size: 1000
      claude:
        pricing:
          {{- range $model, $pricing := .Values.collectorConfig.pricing }}
          {{ $model }}:
            input_per_mtok: {{ $pricing.input_per_mtok }}
            output_per_mtok: {{ $pricing.output_per_mtok }}
          {{- end }}
        team_mapping:
          {{- range $user, $team := .Values.collectorConfig.teamMapping }}
          {{ $user }}: {{ $team }}
          {{- end }}
      contentfilter/dd:
        mode: strip

    exporters:
      datadog:
        api:
          key: ${DD_API_KEY}
          site: {{ .Values.collectorConfig.datadogSite }}

    extensions:
      health_check:
        endpoint: 0.0.0.0:13133

    service:
      extensions: [health_check]
      pipelines:
        traces:
          receivers: [otlp]
          processors: [batch, claude, contentfilter/dd]
          exporters: [datadog]
        metrics:
          receivers: [otlp]
          processors: [batch, claude]
          exporters: [datadog]
```

- [ ] **Step 4: Create secret.yaml**

Create `deploy/helm/claude-otel-collector/templates/secret.yaml`:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Release.Name }}-secrets
  labels:
    app: {{ .Release.Name }}
type: Opaque
stringData:
  DD_API_KEY: {{ .Values.datadogApiKey | quote }}
```

- [ ] **Step 5: Create deployment.yaml**

Create `deploy/helm/claude-otel-collector/templates/deployment.yaml`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
  labels:
    app: {{ .Release.Name }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app: {{ .Release.Name }}
  template:
    metadata:
      labels:
        app: {{ .Release.Name }}
    spec:
      containers:
        - name: collector
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          args:
            - --config
            - /etc/otel/config.yaml
          ports:
            - name: otlp-grpc
              containerPort: 4317
              protocol: TCP
            - name: otlp-http
              containerPort: 4318
              protocol: TCP
            - name: health
              containerPort: 13133
              protocol: TCP
          envFrom:
            - secretRef:
                name: {{ .Release.Name }}-secrets
          volumeMounts:
            - name: config
              mountPath: /etc/otel
              readOnly: true
          livenessProbe:
            httpGet:
              path: /
              port: health
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /
              port: health
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
      volumes:
        - name: config
          configMap:
            name: {{ .Release.Name }}-config
```

- [ ] **Step 6: Create service.yaml**

Create `deploy/helm/claude-otel-collector/templates/service.yaml`:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ .Release.Name }}
  labels:
    app: {{ .Release.Name }}
spec:
  type: {{ .Values.service.type }}
  ports:
    - name: otlp-grpc
      port: {{ .Values.service.grpcPort }}
      targetPort: otlp-grpc
      protocol: TCP
    - name: otlp-http
      port: {{ .Values.service.httpPort }}
      targetPort: otlp-http
      protocol: TCP
  selector:
    app: {{ .Release.Name }}
```

- [ ] **Step 7: Lint the Helm chart**

```bash
helm lint deploy/helm/claude-otel-collector
```

Expected: "1 chart(s) linted, 0 chart(s) failed"

- [ ] **Step 8: Template render check**

```bash
helm template test deploy/helm/claude-otel-collector --set datadogApiKey=test-key
```

Expected: Valid YAML output with all resources rendered correctly.

- [ ] **Step 9: Commit**

```bash
git add deploy/
git commit -m "feat: add Helm chart for K8s deployment"
```

---

### Task 10: End-to-End Smoke Test

**Files:**
- None (validation only)

Verify the full pipeline works locally: send a test OTLP trace, confirm it flows through processors and reaches the debug exporter.

- [ ] **Step 1: Create a test config with debug exporter only**

Create `config/test-config.yaml`:
```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  claude:
    pricing:
      claude-sonnet-4-20250514:
        input_per_mtok: 3.00
        output_per_mtok: 15.00
    team_mapping:
      user-123: platform-team
  contentfilter/dd:
    mode: strip

exporters:
  debug:
    verbosity: detailed

extensions:
  health_check:
    endpoint: 0.0.0.0:13133

service:
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [claude, contentfilter/dd]
      exporters: [debug]
```

- [ ] **Step 2: Start the collector**

```bash
./bin/claude-otel-collector --config config/test-config.yaml &
COLLECTOR_PID=$!
sleep 2
```

- [ ] **Step 3: Send a test trace via grpcurl or telemetrygen**

Install `telemetrygen` and send a test trace:
```bash
go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@v0.115.0
telemetrygen traces --otlp-insecure --traces 1 --otlp-endpoint localhost:4317 \
  --otlp-attributes 'claude.model="claude-sonnet-4-20250514"' \
  --otlp-attributes 'claude.tokens.input="1000"' \
  --otlp-attributes 'claude.tokens.output="500"' \
  --otlp-attributes 'claude.user="user-123"' \
  --otlp-attributes 'claude.content.prompt="test prompt"'
```

- [ ] **Step 4: Check debug exporter output**

Check the collector logs. Expected output should show:
- `claude.cost.usd` attribute is present (cost calculated)
- `claude.team` attribute is `platform-team` (team mapped)
- `claude.content.prompt` is NOT present (stripped by contentfilter)
- `claude.model`, `claude.tokens.input`, etc. are preserved

```bash
kill $COLLECTOR_PID
```

- [ ] **Step 5: Commit test config**

```bash
git add config/test-config.yaml
git commit -m "test: add e2e smoke test config with debug exporter"
```

---

## Self-Review

**Spec coverage:**
- OTLP receiver for Claude Code — Task 7, 10
- `claudeprocessor` (enrichment, cost calc) — Tasks 2, 3, 4
- `contentfilter` (strip mode) — Tasks 5, 6
- DataDog exporter — Task 7
- Single K8s Deployment — Task 9
- Helm chart — Task 9
- Built with `ocb` — Tasks 1, 7

All V1 spec requirements covered.

**Placeholder scan:** No TBDs, TODOs, or vague steps. All code is complete.

**Type consistency:**
- `Config`, `ModelPricing` used consistently in claudeprocessor
- `Config`, `ModeStrip`, `ModeKeep` used consistently in contentfilter
- `newClaudeProcessor` / `newClaudeMetricsProcessor` / `newContentFilterProcessor` consistent between factory and processor files
- Fixed `pCfg` → `mCfg` inconsistency in factory.go Task 3 → Task 4 step 4
