package observability

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
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
	CaptureException(
		ctx,
		errors.New("boom"),
		map[string]string{"source": "test"},
		map[string]interface{}{"attempt": 1},
	)

	events := transport.Events()
	if len(events) != 1 {
		t.Fatalf("expected one Sentry event, got %d", len(events))
	}
	traceContext := events[0].Contexts["trace"]
	if fmt.Sprint(traceContext["trace_id"]) != span.SpanContext().TraceID().String() {
		t.Fatalf("expected trace ID %s, got %#v", span.SpanContext().TraceID(), traceContext)
	}
	if fmt.Sprint(events[0].Contexts["details"]["attempt"]) != "1" {
		t.Fatalf("expected event details, got %#v", events[0].Contexts["details"])
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
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown observability: %v", err)
	}
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	t.Run("trace exporter", func(t *testing.T) {
		t.Setenv("OTEL_TRACES_EXPORTER", "zipkin")
		if _, err := New(context.Background(), "podium"); err == nil {
			t.Fatal("expected invalid trace exporter to fail")
		}
	})

	t.Run("metric exporter", func(t *testing.T) {
		t.Setenv("OTEL_TRACES_EXPORTER", "none")
		t.Setenv("OTEL_METRICS_EXPORTER", "prometheus")
		if _, err := New(context.Background(), "podium"); err == nil {
			t.Fatal("expected invalid metric exporter to fail")
		}
	})

	t.Run("service name", func(t *testing.T) {
		t.Setenv("OTEL_TRACES_EXPORTER", "none")
		t.Setenv("OTEL_METRICS_EXPORTER", "none")
		t.Setenv("OTEL_SERVICE_NAME", " ")
		if _, err := New(context.Background(), ""); err == nil {
			t.Fatal("expected empty service name to fail")
		}
	})

	t.Run("sampler", func(t *testing.T) {
		t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
		t.Setenv("OTEL_METRICS_EXPORTER", "none")
		t.Setenv("OTEL_TRACES_SAMPLER", "unsupported")
		if _, err := New(context.Background(), "podium"); err == nil {
			t.Fatal("expected invalid trace sampler to fail")
		}
	})

	t.Run("Sentry DSN", func(t *testing.T) {
		t.Setenv("OTEL_TRACES_EXPORTER", "none")
		t.Setenv("OTEL_METRICS_EXPORTER", "none")
		t.Setenv("SENTRY_DSN", "://invalid")
		if _, err := New(context.Background(), "podium"); err == nil {
			t.Fatal("expected invalid Sentry DSN to fail")
		}
	})
}

func TestNewWithOTLPExporters(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
	t.Setenv("OTEL_METRICS_EXPORTER", "otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	t.Setenv("SENTRY_DSN", "")

	previousTracerProvider := otel.GetTracerProvider()
	previousMeterProvider := otel.GetMeterProvider()
	previousPropagator := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(previousTracerProvider)
		otel.SetMeterProvider(previousMeterProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})

	provider, err := New(context.Background(), "podium")
	if err != nil {
		t.Fatalf("initialize observability: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := provider.Shutdown(ctx); err == nil {
		t.Fatal("expected canceled shutdown to report exporter errors")
	}
}

func TestProviderShutdown(t *testing.T) {
	provider := &Provider{
		tracerProvider: sdktrace.NewTracerProvider(),
		meterProvider:  metric.NewMeterProvider(),
	}

	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown providers: %v", err)
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

	t.Setenv("OTEL_TRACES_EXPORTER", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	traceEnabled, err = otlpExportEnabled(
		"OTEL_TRACES_EXPORTER",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
	)
	if err != nil {
		t.Fatalf("resolve unconfigured trace exporter: %v", err)
	}
	if traceEnabled {
		t.Fatal("expected unconfigured trace export to be disabled")
	}

	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "localhost:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	traceEnabled, err = otlpExportEnabled(
		"OTEL_TRACES_EXPORTER",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
	)
	if err != nil {
		t.Fatalf("resolve shared OTLP protocol: %v", err)
	}
	if !traceEnabled {
		t.Fatal("expected trace endpoint to enable export")
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
	tests := []struct {
		name     string
		sampler  string
		ratio    string
		decision sdktrace.SamplingDecision
	}{
		{name: "default", sampler: "", decision: sdktrace.RecordAndSample},
		{name: "always on", sampler: "always_on", decision: sdktrace.RecordAndSample},
		{name: "always off", sampler: "always_off", decision: sdktrace.Drop},
		{name: "parent based always off", sampler: "parentbased_always_off", decision: sdktrace.Drop},
		{name: "ratio", sampler: "traceidratio", ratio: "0", decision: sdktrace.Drop},
		{name: "parent based ratio", sampler: "parentbased_traceidratio", ratio: "0", decision: sdktrace.Drop},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER", tt.sampler)
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", tt.ratio)
			sampler, err := traceSampler()
			if err != nil {
				t.Fatalf("create sampler: %v", err)
			}
			if got := sampler.ShouldSample(sdktrace.SamplingParameters{}).Decision; got != tt.decision {
				t.Fatalf("expected decision %v, got %v", tt.decision, got)
			}
		})
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
