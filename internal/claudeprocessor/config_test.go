package claudeprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := &Config{
		Pricing: map[string]ModelPricing{
			"claude-sonnet-4-20250514": {InputPerMTok: 3.00, OutputPerMTok: 15.00},
		},
	}
	require.NoError(t, cfg.Validate())
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
			"claude-sonnet-4-20250514": {InputPerMTok: -1.0, OutputPerMTok: 15.00},
		},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "negative")
}

func TestConfig_Validate_WithTeamMapping(t *testing.T) {
	cfg := &Config{
		Pricing: map[string]ModelPricing{
			"claude-sonnet-4-20250514": {InputPerMTok: 3.00, OutputPerMTok: 15.00},
		},
		TeamMapping: map[string]string{"user-123": "platform-team"},
	}
	require.NoError(t, cfg.Validate())
}
