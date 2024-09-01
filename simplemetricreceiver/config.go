package simplemetricreceiver

import (
	"fmt"
	"time"
)

type Config struct {
	Interval string `mapstructure:"interval"`
}

func (cfg *Config) Validate() error {
	interval, _ := time.ParseDuration(cfg.Interval)
	if interval.Seconds() < 2 {
		return fmt.Errorf("when defined, the interval must be set to at least 2 seconds")
	}
	return nil
}
