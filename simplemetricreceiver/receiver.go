package simplemetricreceiver

import (
	"context"
	"math/rand"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type simplemetricReceiver struct {
	host          component.Host
	cancel        context.CancelFunc
	logger        *zap.Logger
	nextConsumer  consumer.Metrics
	config        *Config
	simpleGauge   int64
	simpleCounter int64
}

func (smr *simplemetricReceiver) scrapeMetrics() {
	smr.simpleGauge = rand.Int63n(10)
	smr.simpleCounter += smr.simpleGauge
	smr.logger.Sugar().Infof("Scraped metrics: gauge %d, counter %d", smr.simpleGauge, smr.simpleCounter)
}

func (smr *simplemetricReceiver) Start(ctx context.Context, host component.Host) error {
	smr.host = host
	ctx = context.Background()
	ctx, smr.cancel = context.WithCancel(ctx)
	interval, _ := time.ParseDuration(smr.config.Interval)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				smr.logger.Info("I should start processing metrics now!")
				ts := pcommon.NewTimestampFromTime(time.Now())
				// scrape metrics
				smr.scrapeMetrics()
				// place holder to report multiple new metrics
				metrics := pmetric.NewMetrics()
				// simplecounter
				m1 := pmetric.NewMetric()
				m1.SetName("simplecounter")
				m1.SetDescription("A Simple Counter")
				m1.SetUnit("unit")
				var dp1 pmetric.NumberDataPoint
				sum := m1.SetEmptySum()
				sum.SetIsMonotonic(true)
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp1 = sum.DataPoints().AppendEmpty()
				dp1.SetTimestamp(ts)
				dp1.SetStartTimestamp(ts)
				dp1.SetIntValue(smr.simpleCounter)
				// append to metrics
				newMetric1 := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
				m1.MoveTo(newMetric1)
				//simplegauge
				m2 := pmetric.NewMetric()
				m2.SetName("simplegauge")
				m2.SetDescription("A Simple Gauge")
				m2.SetUnit("unit")
				var dp2 pmetric.NumberDataPoint
				dp2 = m2.SetEmptyGauge().DataPoints().AppendEmpty()
				dp2.SetTimestamp(ts)
				dp2.SetStartTimestamp(ts)
				dp2.SetIntValue(smr.simpleGauge)
				// append to metrics
				newMetric2 := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
				m2.MoveTo(newMetric2)
				//consume
				err := smr.nextConsumer.ConsumeMetrics(ctx, metrics)
				if err != nil {
					smr.logger.Error(err.Error())
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}
func (smr *simplemetricReceiver) Shutdown(ctx context.Context) error {
	if smr.cancel != nil {
		smr.cancel()
	}
	return nil
}
