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

	attrs := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
	_, hasPrompt := attrs.Get("claude.content.prompt")
	_, hasResponse := attrs.Get("claude.content.response")
	assert.False(t, hasPrompt, "prompt should be stripped")
	assert.False(t, hasResponse, "response should be stripped")

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

	attrs := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()
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
