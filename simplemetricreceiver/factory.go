package simplemetricreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var (
	typeStr = component.MustNewType("simplemetricreceiver")
)

const (
	defaultInterval = 5 * time.Second
)

func createDefaultConfig() component.Config {
	return &Config{
		Interval: string(defaultInterval),
	}
}

func createMetricReceiver(_ context.Context, params receiver.Settings, baseCfg component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	logger := params.Logger
	smrCfg := baseCfg.(*Config)
	simplemetricReceiver := &simplemetricReceiver{
		logger:       logger,
		nextConsumer: consumer,
		config:       smrCfg,
	}
	return simplemetricReceiver, nil
}

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricReceiver, component.StabilityLevelAlpha))
}
