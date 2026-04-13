package claudeprocessor

import (
	"errors"
	"fmt"
)

type Config struct {
	Pricing     map[string]ModelPricing `mapstructure:"pricing"`
	TeamMapping map[string]string       `mapstructure:"team_mapping"`
}

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
