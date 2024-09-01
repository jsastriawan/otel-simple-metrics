# Open Telemetry Simple Metrics receiver example

## Description

This repository documents the process of developing custom Open Telemetry Collector and then building a custom metrics receiver.

## Building custom Open Telemetry Collector

See https://opentelemetry.io/docs/collector/custom-collector/

Download Open Telemetry Collector builder (e.g. v0.108.o release):
https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/cmd%2Fbuilder%2Fv0.108.0/ocb_0.108.0_linux_amd64

Use the following builder-config.yml

```yml
dist:
  name: otelcol-simple
  description: Basic OTel Collector distribution for Metrics exporter
  output_path: ./otelcol-simple
  otelcol_version: 0.108.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/debugexporter v0.108.0
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.108.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.108.0

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.108.0

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/otlpreceiver v0.108.0

```

This will build a simple collector with debugm OTLP and Prometheus exporters, batch processor and OTLP receiver.

Then run:

```bash
ocb --config builder-config.yml

```

It should download build the main code for collector, pull the required dependencies and then compile an executable binary named "otelcol-simple".
To test this custom otel binary, we can use a simple configuration as following:

```yml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
  

processors:
  batch:

exporters:
  debug:
    verbosity: detailed
  prometheus:
    endpoint: 0.0.0.0:9100

service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug, prometheus]
  telemetry:
    logs:
      level: debug

```

To run otelcol-simple with this config:

```bash
./otelcol-simple --config config-simple.yml
```

We can test if it works with curl by checking if http://localhost:9100 is responding to HTTP request.

```bash
$ curl -s http://localhost:9100/
404 page not found
$ curl -s http://localhost:9100/metrics
```

This means Prometheus exporter is listening as configured.

## Creating custom metric receiver

Let's create a folder named simplemetric receiver and initialize it appropriately

```bash
$ go mod init github.com/jsastriawan/otel-simple-metrics/simplemetricreceiver
go: creating new go.mod: module github.com/jsastriawan/otel-simple-metrics/simplemetricreceiver
```

This simple metric receiver basically will periodically report two metric, consist of one Gauge which shows random integer and one Counter which accumulates Gauge reading.

So, the appropriate configuration will be:

```yml
receivers:
  simplemetricreceiver:
    interval: 5s
```

This means simplemetricreceiver will collect the metric every 5 seconds. This translates to skeleton configuraton (config.go) code as following:

```
package simplemetricreceiver

type Config struct {
	Interval string `mapstructure:"interval"`
}

```

To ensure this receiver reports metric at reasonable rate, we shall define a validator function not allowing Interval to be less than 2 second.

```
func (cfg *Config) Validate() error {
	interval, _ := time.ParseDuration(cfg.Interval)
	if interval.Seconds() < 2 {
		return fmt.Errorf("when defined, the interval must be set to at least 2 seconds")
	}
	return nil
}
```

Next, we shall implement factory interface by defining NewFactory method for this receiver. Since this is a Metric Receiver, the NewFactory must have receiver.WithMetric to signify that this module will provide metric receiver capability. Below is the fragment of factory.go:

```
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

```

Next, we shall implement the receiver component itself. The receiver object must implement Start and Shutdown methods.

```
Start(ctx context.Context, host Host) error
Shutdown(ctx context.Context) error
```

But before we go further, lets define the basic capabilities of receiver. Let's define simplemetricReceiver struct which contains several members and two integer metrics. These two integer metrics will be updated everytime scrape event happens. 

```
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

```

When this receiver is activated by OpenTelemetry Collector, this shall start a timer and then scrape and consume metrics when timer elapsed. When this receiver is shutdown, the timer should be cancelled appropriately. As a result, the tow methods will be like below:

```
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
```

Please also note how Gauge and Counter is handled differently.
Counter requires monotonic and aggregation temporality to be defined whereas Gauge is just a simple datapoint append to Metrics.

Additionally, timestamp, description and unit are important properties to populate for better clarity.

## Adding simplemetricreceiver to Open Telemetry Collector

First, to ensure the visibility of simplemetricreceiver package is known, it should be installed.

```
cd simplemetricreceiver
go install
```

If the source code is not published to git repo, then we should modify the otelcol-simple/go.mod to replace the package reference to local path.

```
replace github.com/jsastriawan/otel-simple-metrics/simplemetricreceiver => ../simplemetricreceiver
```

then we can add the simplemetricreceiver to otelcolsimple

```
go get github.com/jsastriawan/otel-simple-metrics/simplemetricreceiver
```

This will update go.mod appropriately.

In order to registrer simplemetricreceiver to otelcol-simple, we shall import this package.
```
import (
	
	...
	simplemetricreceiver "github.com/jsastriawan/otel-simple-metrics/simplemetricreceiver"
)
```
Then, it shall be added to Receivers factory map and register the module.

```
factories.Receivers, err = receiver.MakeFactoryMap(
		otlpreceiver.NewFactory(),
		simplemetricreceiver.NewFactory(),
	)

....
factories.ReceiverModules[simplemetricreceiver.NewFactory().Type()] = "github.com/jsastriawan/otel-simple-metrics/simplemetricreceiver v0.0.0-00010101000000-000000000000"
```
## Running and observing

Once the above steps are completed, we can add this receiver into new Open Telemetry Collector configuration (config-simple.yml).

```
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
  simplemetricreceiver:
    interval: 5s
  

processors:
  batch:

exporters:
  debug:
    verbosity: detailed
  prometheus:
    endpoint: 0.0.0.0:9100

service:
  pipelines:
    metrics:
      receivers: [otlp, simplemetricreceiver]
      processors: [batch]
      exporters: [debug, prometheus]
  telemetry:
    logs:
      level: debug

```

To run this, it can be compiled to binary or just us _go run_ command:
```
go run . --config config-simple.yml
```
We should see the debug log printing the receiver activities.
The metrics produced by this receiver can be observed also via Prometheus exporter using curl:

```
$ curl -s http://localhost:9100/metrics
# HELP simplecounter_unit_total A Simple Counter
# TYPE simplecounter_unit_total counter
simplecounter_unit_total 7
# HELP simplegauge_unit A Simple Gauge
# TYPE simplegauge_unit gauge
simplegauge_unit 5
```
