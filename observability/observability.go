package observability

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	sentryotel "github.com/getsentry/sentry-go/otel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

// Provider owns the process-wide OpenTelemetry and Sentry lifecycle.
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *metric.MeterProvider
	shutdownOnce   sync.Once
	shutdownErr    error
}

// New configures observability from the standard OTEL_* and SENTRY_* environment variables.
func New(ctx context.Context, defaultServiceName string) (*Provider, error) {
	traceEnabled, err := otlpExportEnabled(
		"OTEL_TRACES_EXPORTER",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
	)
	if err != nil {
		return nil, err
	}
	metricEnabled, err := otlpExportEnabled(
		"OTEL_METRICS_EXPORTER",
		"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
	)
	if err != nil {
		return nil, err
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = defaultServiceName
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return nil, errors.New("OpenTelemetry service name must not be empty")
	}

	resource, err := sdkresource.Merge(
		sdkresource.Default(),
		sdkresource.NewSchemaless(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	provider := &Provider{}
	if traceEnabled {
		sampler, err := traceSampler()
		if err != nil {
			return nil, err
		}
		exporter, err := otlptracegrpc.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
		}
		provider.tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(resource),
			sdktrace.WithSampler(sampler),
		)
		otel.SetTracerProvider(provider.tracerProvider)
	}

	if metricEnabled {
		exporter, err := otlpmetricgrpc.New(ctx)
		if err != nil {
			_ = provider.Shutdown(ctx)
			return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
		}
		provider.meterProvider = metric.NewMeterProvider(
			metric.WithReader(metric.NewPeriodicReader(exporter)),
			metric.WithResource(resource),
		)
		otel.SetMeterProvider(provider.meterProvider)
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              os.Getenv("SENTRY_DSN"),
		Environment:      os.Getenv("SENTRY_ENVIRONMENT"),
		Release:          os.Getenv("SENTRY_RELEASE"),
		EnableTracing:    false,
		AttachStacktrace: true,
		DisableLogs:      true,
		DisableMetrics:   true,
		Integrations:     sentryIntegrations,
	}); err != nil {
		_ = provider.Shutdown(ctx)
		return nil, fmt.Errorf("initialize Sentry: %w", err)
	}

	return provider, nil
}

func sentryIntegrations(integrations []sentry.Integration) []sentry.Integration {
	return append(integrations, sentryotel.NewOtelIntegration())
}

// Shutdown flushes pending telemetry and stops the configured exporters.
func (p *Provider) Shutdown(ctx context.Context) error {
	p.shutdownOnce.Do(func() {
		var errs []error
		if !sentry.Flush(2 * time.Second) {
			errs = append(errs, errors.New("flush Sentry events"))
		}
		if p.meterProvider != nil {
			if err := p.meterProvider.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("shutdown metric provider: %w", err))
			}
		}
		if p.tracerProvider != nil {
			if err := p.tracerProvider.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("shutdown tracer provider: %w", err))
			}
		}
		p.shutdownErr = errors.Join(errs...)
	})
	return p.shutdownErr
}

// CaptureException reports an error to Sentry and correlates it with the active OpenTelemetry span.
func CaptureException(ctx context.Context, err error, tags map[string]string, details map[string]interface{}) {
	hub := sentry.CurrentHub().Clone()
	hub.WithScope(func(scope *sentry.Scope) {
		for key, value := range tags {
			scope.SetTag(key, value)
		}
		if len(details) > 0 {
			scope.SetContext("details", details)
		}
		client := hub.Client()
		if client != nil {
			client.CaptureException(err, &sentry.EventHint{Context: ctx}, scope)
		}
	})
}

func otlpExportEnabled(exporterName, protocolName string, endpointNames ...string) (bool, error) {
	exporter := strings.ToLower(strings.TrimSpace(os.Getenv(exporterName)))
	switch exporter {
	case "none":
		return false, nil
	case "", "otlp":
	default:
		return false, fmt.Errorf("%s=%q is unsupported; use otlp or none", exporterName, exporter)
	}

	enabled := exporter == "otlp"
	for _, name := range endpointNames {
		if os.Getenv(name) != "" {
			enabled = true
			break
		}
	}
	if !enabled {
		return false, nil
	}

	protocol := strings.ToLower(strings.TrimSpace(os.Getenv(protocolName)))
	protocolSource := protocolName
	if protocol == "" {
		protocol = strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")))
		protocolSource = "OTEL_EXPORTER_OTLP_PROTOCOL"
	}
	if protocol != "" && protocol != "grpc" {
		return false, fmt.Errorf("%s=%q is unsupported; Podium exports OTLP over gRPC", protocolSource, protocol)
	}
	return true, nil
}

func traceSampler() (sdktrace.Sampler, error) {
	samplerName := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER")))
	switch samplerName {
	case "", "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample()), nil
	case "always_on":
		return sdktrace.AlwaysSample(), nil
	case "always_off":
		return sdktrace.NeverSample(), nil
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample()), nil
	case "traceidratio":
		ratio, err := traceSampleRatio()
		if err != nil {
			return nil, err
		}
		return sdktrace.TraceIDRatioBased(ratio), nil
	case "parentbased_traceidratio":
		ratio, err := traceSampleRatio()
		if err != nil {
			return nil, err
		}
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio)), nil
	default:
		return nil, fmt.Errorf("OTEL_TRACES_SAMPLER=%q is unsupported", samplerName)
	}
}

func traceSampleRatio() (float64, error) {
	value := os.Getenv("OTEL_TRACES_SAMPLER_ARG")
	ratio, err := strconv.ParseFloat(value, 64)
	if err != nil || ratio < 0 || ratio > 1 {
		return 0, fmt.Errorf("OTEL_TRACES_SAMPLER_ARG=%q must be a number between 0 and 1", value)
	}
	return ratio, nil
}
