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
			"claude-sonnet-4-20250514": {InputPerMTok: 3.00, OutputPerMTok: 15.00},
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
	assert.InDelta(t, 0.0105, costVal.Double(), 0.0001)
}

func TestProcessor_UnknownModel(t *testing.T) {
	cfg := &Config{
		Pricing: map[string]ModelPricing{
			"claude-sonnet-4-20250514": {InputPerMTok: 3.00, OutputPerMTok: 15.00},
		},
	}
	sink := &consumertest.TracesSink{}
	proc := newClaudeProcessor(zap.NewNop(), cfg, sink)

	td := newTestTraces("unknown-model", 1000, 500, "")
	err := proc.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)
	_, ok := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Get("claude.cost.usd")
	assert.False(t, ok)
}

func TestProcessor_TeamMapping(t *testing.T) {
	cfg := &Config{
		Pricing: map[string]ModelPricing{
			"claude-sonnet-4-20250514": {InputPerMTok: 3.00, OutputPerMTok: 15.00},
		},
		TeamMapping: map[string]string{"user-123": "platform-team"},
	}
	sink := &consumertest.TracesSink{}
	proc := newClaudeProcessor(zap.NewNop(), cfg, sink)

	td := newTestTraces("claude-sonnet-4-20250514", 1000, 500, "user-123")
	err := proc.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)

	teamVal, ok := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Get("claude.team")
	require.True(t, ok)
	assert.Equal(t, "platform-team", teamVal.Str())
}

func TestProcessor_NoTeamMapping(t *testing.T) {
	cfg := &Config{
		Pricing: map[string]ModelPricing{
			"claude-sonnet-4-20250514": {InputPerMTok: 3.00, OutputPerMTok: 15.00},
		},
		TeamMapping: map[string]string{"user-123": "platform-team"},
	}
	sink := &consumertest.TracesSink{}
	proc := newClaudeProcessor(zap.NewNop(), cfg, sink)

	td := newTestTraces("claude-sonnet-4-20250514", 1000, 500, "user-999")
	err := proc.ConsumeTraces(context.Background(), td)
	require.NoError(t, err)
	_, ok := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Get("claude.team")
	assert.False(t, ok)
}
