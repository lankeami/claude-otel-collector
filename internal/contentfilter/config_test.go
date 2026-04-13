package contentfilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_Strip(t *testing.T) {
	cfg := &Config{Mode: ModeStrip}
	require.NoError(t, cfg.Validate())
}

func TestConfig_Validate_Keep(t *testing.T) {
	cfg := &Config{Mode: ModeKeep}
	require.NoError(t, cfg.Validate())
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
