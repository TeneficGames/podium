package enriching

import (
	"context"
	"fmt"
	"time"

	"github.com/TeneficGames/podium/leaderboard/model"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	enrichmentDuration = "podium.enrichment.duration"
	enrichmentCalls    = "podium.enrichment.calls"
	enrichmentErrors   = "podium.enrichment.errors"
)

type instrumentedEnricher struct {
	impl     Enricher
	calls    metric.Int64Counter
	errors   metric.Int64Counter
	duration metric.Float64Histogram
}

func NewInstrumentedEnricher(impl Enricher) (Enricher, error) {
	meter := otel.Meter("github.com/TeneficGames/podium/leaderboard/enriching")
	calls, err := meter.Int64Counter(enrichmentCalls)
	if err != nil {
		return nil, fmt.Errorf("create enrichment calls counter: %w", err)
	}
	errors, err := meter.Int64Counter(enrichmentErrors)
	if err != nil {
		return nil, fmt.Errorf("create enrichment errors counter: %w", err)
	}
	duration, err := meter.Float64Histogram(
		enrichmentDuration,
		metric.WithUnit("s"),
		metric.WithDescription("Duration of leaderboard enrichment"),
	)
	if err != nil {
		return nil, fmt.Errorf("create enrichment duration histogram: %w", err)
	}
	return &instrumentedEnricher{
		impl:     impl,
		calls:    calls,
		errors:   errors,
		duration: duration,
	}, nil
}

func (en *instrumentedEnricher) Enrich(ctx context.Context, tenantID, leaderboardID string, members []*model.Member) ([]*model.Member, error) {
	start := time.Now()

	ctx, span := otel.Tracer("github.com/TeneficGames/podium/leaderboard/enriching").Start(
		ctx,
		"podium.enriching",
		trace.WithAttributes(
			attribute.String("tenant.id", tenantID),
			attribute.String("leaderboard.id", leaderboardID),
		),
	)
	defer span.End()

	members, err := en.impl.Enrich(ctx, tenantID, leaderboardID, members)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		en.errors.Add(ctx, 1)
	}

	en.calls.Add(ctx, 1)
	en.duration.Record(ctx, time.Since(start).Seconds())

	return members, err
}

var _ Enricher = (*instrumentedEnricher)(nil)
