package claudeprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type claudeProcessor struct {
	logger     *zap.Logger
	cfg        *Config
	nextTraces consumer.Traces
}

func newClaudeProcessor(logger *zap.Logger, cfg *Config, next consumer.Traces) *claudeProcessor {
	return &claudeProcessor{logger: logger, cfg: cfg, nextTraces: next}
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

	modelVal, ok := attrs.Get("claude.model")
	if ok {
		model := modelVal.Str()
		if pricing, found := p.cfg.Pricing[model]; found {
			var inputTokens, outputTokens int64
			if v, ok := attrs.Get("claude.tokens.input"); ok {
				inputTokens = v.Int()
			}
			if v, ok := attrs.Get("claude.tokens.output"); ok {
				outputTokens = v.Int()
			}
			cost := (float64(inputTokens)*pricing.InputPerMTok/1_000_000) +
				(float64(outputTokens)*pricing.OutputPerMTok/1_000_000)
			attrs.PutDouble("claude.cost.usd", cost)
		} else {
			p.logger.Debug("no pricing found for model", zap.String("model", model))
		}
	}

	if userVal, ok := attrs.Get("claude.user"); ok {
		if team, found := p.cfg.TeamMapping[userVal.Str()]; found {
			attrs.PutStr("claude.team", team)
		}
	}
}

func (p *claudeProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *claudeProcessor) Start(_ context.Context, _ component.Host) error { return nil }
func (p *claudeProcessor) Shutdown(_ context.Context) error                 { return nil }
