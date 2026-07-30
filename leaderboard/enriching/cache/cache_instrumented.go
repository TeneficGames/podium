package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/TeneficGames/podium/leaderboard/enriching"
	"github.com/TeneficGames/podium/leaderboard/model"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	enrichmentCacheGetDuration = "podium.enrichment.cache.get.duration"
	enrichmentCacheGets        = "podium.enrichment.cache.gets"
	enrichmentCacheHits        = "podium.enrichment.cache.hits"
	enrichmentCacheGetErrors   = "podium.enrichment.cache.get.errors"
	enrichmentCacheSetDuration = "podium.enrichment.cache.set.duration"
	enrichmentCacheSets        = "podium.enrichment.cache.sets"
	enrichmentCacheSetErrors   = "podium.enrichment.cache.set.errors"
)

type instrumentedCache struct {
	impl      enriching.EnricherCache
	gets      metric.Int64Counter
	hits      metric.Int64Counter
	getErrors metric.Int64Counter
	getTime   metric.Float64Histogram
	sets      metric.Int64Counter
	setErrors metric.Int64Counter
	setTime   metric.Float64Histogram
}

// NewInstrumentedCache returns an EnrichCache implementation wrapped
// with metrics reporting and tracing
func NewInstrumentedCache(impl enriching.EnricherCache) (enriching.EnricherCache, error) {
	meter := otel.Meter("github.com/TeneficGames/podium/leaderboard/enriching/cache")
	gets, err := meter.Int64Counter(enrichmentCacheGets)
	if err != nil {
		return nil, fmt.Errorf("create enrichment cache gets counter: %w", err)
	}
	hits, err := meter.Int64Counter(enrichmentCacheHits)
	if err != nil {
		return nil, fmt.Errorf("create enrichment cache hits counter: %w", err)
	}
	getErrors, err := meter.Int64Counter(enrichmentCacheGetErrors)
	if err != nil {
		return nil, fmt.Errorf("create enrichment cache get errors counter: %w", err)
	}
	getTime, err := meter.Float64Histogram(enrichmentCacheGetDuration, metric.WithUnit("s"))
	if err != nil {
		return nil, fmt.Errorf("create enrichment cache get duration histogram: %w", err)
	}
	sets, err := meter.Int64Counter(enrichmentCacheSets)
	if err != nil {
		return nil, fmt.Errorf("create enrichment cache sets counter: %w", err)
	}
	setErrors, err := meter.Int64Counter(enrichmentCacheSetErrors)
	if err != nil {
		return nil, fmt.Errorf("create enrichment cache set errors counter: %w", err)
	}
	setTime, err := meter.Float64Histogram(enrichmentCacheSetDuration, metric.WithUnit("s"))
	if err != nil {
		return nil, fmt.Errorf("create enrichment cache set duration histogram: %w", err)
	}
	return &instrumentedCache{
		impl:      impl,
		gets:      gets,
		hits:      hits,
		getErrors: getErrors,
		getTime:   getTime,
		sets:      sets,
		setErrors: setErrors,
		setTime:   setTime,
	}, nil
}

func (c *instrumentedCache) Get(
	ctx context.Context,
	tenantID,
	leaderboardID string,
	members []*model.Member,
) (map[string]map[string]string, bool, error) {
	start := time.Now()

	ctx, span := otel.Tracer("github.com/TeneficGames/podium/leaderboard/enriching/cache").Start(
		ctx,
		"podium.enriching_cache.get",
		trace.WithAttributes(
			attribute.String("tenant.id", tenantID),
			attribute.String("leaderboard.id", leaderboardID),
		),
	)
	defer span.End()

	metadata, hit, err := c.impl.Get(ctx, tenantID, leaderboardID, members)

	c.gets.Add(ctx, 1)
	c.getTime.Record(ctx, time.Since(start).Seconds())

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.getErrors.Add(ctx, 1)
	}

	if hit {
		c.hits.Add(ctx, 1)
	}

	return metadata, hit, err
}

func (c *instrumentedCache) Set(
	ctx context.Context,
	tenantID,
	leaderboardID string,
	members []*model.Member,
	ttl time.Duration,
) error {
	start := time.Now()

	ctx, span := otel.Tracer("github.com/TeneficGames/podium/leaderboard/enriching/cache").Start(
		ctx,
		"podium.enriching_cache.set",
		trace.WithAttributes(
			attribute.String("tenant.id", tenantID),
			attribute.String("leaderboard.id", leaderboardID),
			attribute.Int64("cache.ttl_seconds", int64(ttl.Seconds())),
		),
	)
	defer span.End()

	err := c.impl.Set(ctx, tenantID, leaderboardID, members, ttl)

	c.sets.Add(ctx, 1)
	c.setTime.Record(ctx, time.Since(start).Seconds())

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.setErrors.Add(ctx, 1)
	}

	return err
}

var _ enriching.EnricherCache = &instrumentedCache{}
