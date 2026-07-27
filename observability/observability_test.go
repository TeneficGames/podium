package observability

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestCaptureExceptionIncludesTraceContext(t *testing.T) {
	transport := &sentry.MockTransport{}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:            "https://public@example.com/1",
		Transport:      transport,
		DisableLogs:    true,
		DisableMetrics: true,
		Integrations:   sentryIntegrations,
	}); err != nil {
		t.Fatalf("initialize Sentry: %v", err)
	}

	previousProvider := otel.GetTracerProvider()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(tracerProvider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		_ = tracerProvider.Shutdown(context.Background())
	})

	ctx, span := otel.Tracer("test").Start(context.Background(), "test")
	defer span.End()
	CaptureException(ctx, errors.New("boom"), map[string]string{"source": "test"}, nil)

	events := transport.Events()
	if len(events) != 1 {
		t.Fatalf("expected one Sentry event, got %d", len(events))
	}
	traceContext := events[0].Contexts["trace"]
	if fmt.Sprint(traceContext["trace_id"]) != span.SpanContext().TraceID().String() {
		t.Fatalf("expected trace ID %s, got %#v", span.SpanContext().TraceID(), traceContext)
	}
}

func TestNewWithExportersDisabled(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	t.Setenv("SENTRY_DSN", "")

	provider, err := New(context.Background(), "podium")
	if err != nil {
		t.Fatalf("initialize observability: %v", err)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown observability: %v", err)
	}
}

func TestExportEnabled(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

	traceEnabled, err := otlpExportEnabled(
		"OTEL_TRACES_EXPORTER",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
	)
	if err != nil {
		t.Fatalf("resolve trace exporter: %v", err)
	}
	if !traceEnabled {
		t.Fatal("expected trace export to be enabled")
	}

	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	traceEnabled, err = otlpExportEnabled(
		"OTEL_TRACES_EXPORTER",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
	)
	if err != nil {
		t.Fatalf("resolve disabled trace exporter: %v", err)
	}
	if traceEnabled {
		t.Fatal("expected trace export to be disabled")
	}
}

func TestExportConfigurationErrors(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "zipkin")
	_, err := otlpExportEnabled("OTEL_TRACES_EXPORTER", "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL")
	if err == nil {
		t.Fatal("expected unsupported exporter to fail")
	}

	t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "http/protobuf")
	_, err = otlpExportEnabled("OTEL_TRACES_EXPORTER", "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL")
	if err == nil {
		t.Fatal("expected unsupported protocol to fail")
	}
}

func TestTraceSampler(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER", "always_off")
	sampler, err := traceSampler()
	if err != nil {
		t.Fatalf("create sampler: %v", err)
	}
	if sampler.ShouldSample(sdktrace.SamplingParameters{}).Decision != sdktrace.Drop {
		t.Fatal("expected always_off sampler to drop")
	}

	t.Setenv("OTEL_TRACES_SAMPLER", "traceidratio")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "0")
	sampler, err = traceSampler()
	if err != nil {
		t.Fatalf("create ratio sampler: %v", err)
	}
	if sampler.ShouldSample(sdktrace.SamplingParameters{}).Decision != sdktrace.Drop {
		t.Fatal("expected zero ratio sampler to drop")
	}
}

func TestTraceSamplerRejectsInvalidConfiguration(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER", "parentbased_traceidratio")
	t.Setenv("OTEL_TRACES_SAMPLER_ARG", "2")
	if _, err := traceSampler(); err == nil {
		t.Fatal("expected invalid ratio to fail")
	}

	t.Setenv("OTEL_TRACES_SAMPLER", "unsupported")
	if _, err := traceSampler(); err == nil {
		t.Fatal("expected unsupported sampler to fail")
	}
}
