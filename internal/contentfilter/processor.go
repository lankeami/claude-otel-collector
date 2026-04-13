package contentfilter

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
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
	return &contentFilterProcessor{logger: logger, cfg: cfg, next: next}
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
	var toRemove []string
	attrs.Range(func(k string, _ pcommon.Value) bool {
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

func (p *contentFilterProcessor) Start(_ context.Context, _ component.Host) error { return nil }
func (p *contentFilterProcessor) Shutdown(_ context.Context) error                 { return nil }
