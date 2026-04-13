package contentfilter

import "fmt"

const (
	ModeStrip = "strip"
	ModeKeep  = "keep"
)

type Config struct {
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
