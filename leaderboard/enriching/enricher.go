package enriching

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	podium_enrichment_v1 "github.com/TeneficGames/podium/leaderboard/enriching/proto/enrichment/v1"
	"github.com/TeneficGames/podium/leaderboard/model"
	"go.uber.org/zap"
)

type enricherImpl struct {
	config enrichmentConfig
	logger *zap.Logger
	client *http.Client
}

// NewEnricher returns a new Enricher implementation.
func NewEnricher(options ...EnricherOptions) Enricher {
	e := &enricherImpl{
		config: newDefaultEnrichConfig(),
		logger: zap.NewNop(),
	}

	for _, opt := range options {
		opt(e)
	}

	e.client = &http.Client{Timeout: e.config.requestTimeout}
	return e
}

// Enrich adds metadata from the HTTP provider configured for the tenant.
func (e *enricherImpl) Enrich(
	ctx context.Context,
	tenantID,
	leaderboardID string,
	members []*model.Member,
) ([]*model.Member, error) {
	if len(members) == 0 {
		return members, nil
	}

	provider, exists := e.config.providers[tenantID]
	if !exists || provider.Endpoint == "" {
		return members, nil
	}

	enriched, err := e.enrichWithHTTP(ctx, provider, tenantID, leaderboardID, members)
	if err != nil {
		e.logger.Error(
			"could not enrich members",
			zap.String("tenantID", tenantID),
			zap.String("leaderboardID", leaderboardID),
			zap.Error(err),
		)
		return members, &ProviderError{
			Strict: provider.Mode == FailureModeStrict,
			Err:    err,
		}
	}

	return enriched, nil
}

func (e *enricherImpl) enrichWithHTTP(
	ctx context.Context,
	provider Provider,
	tenantID,
	leaderboardID string,
	members []*model.Member,
) ([]*model.Member, error) {
	endpoint, err := validateEndpoint(provider.Endpoint)
	if err != nil {
		return members, fmt.Errorf("invalid enrichment provider endpoint: %w", errors.Join(err, ErrEnrichmentInternal))
	}

	body := make([]*podium_enrichment_v1.Member, len(members))
	for i, member := range members {
		body[i] = &podium_enrichment_v1.Member{
			Id:    member.PublicID,
			Score: member.Score,
			Rank:  int32(member.Rank),
		}
	}

	jsonData, err := json.Marshal(&podium_enrichment_v1.EnrichMembersRequest{
		TenantId:      tenantID,
		LeaderboardId: leaderboardID,
		Members:       body,
	})
	if err != nil {
		return members, fmt.Errorf("marshal enrichment request: %w", errors.Join(err, ErrEnrichmentInternal))
	}

	resp, err := e.callProvider(ctx, endpoint, provider, jsonData)
	if err != nil {
		return members, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var result podium_enrichment_v1.EnrichMembersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return members, fmt.Errorf("decode enrichment response: %w", errors.Join(err, ErrEnrichmentCall))
	}

	metadataByMember := make(map[string]map[string]string, len(result.Members))
	for _, member := range result.Members {
		metadataByMember[member.Id] = member.Metadata
	}

	for _, member := range members {
		if metadata, ok := metadataByMember[member.PublicID]; ok {
			member.Metadata = metadata
		}
	}

	return members, nil
}

func (e *enricherImpl) callProvider(
	ctx context.Context,
	endpoint string,
	provider Provider,
	body []byte,
) (*http.Response, error) {
	maxAttempts := provider.Retry.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	backoff := provider.Retry.InitialBackoff

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create enrichment request: %w", errors.Join(err, ErrEnrichmentInternal))
		}
		req.Header.Set("Content-Type", "application/json")
		for key, value := range provider.Headers {
			req.Header.Set(key, value)
		}

		resp, err := e.client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		retryable := err != nil || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if !retryable || attempt == maxAttempts || ctx.Err() != nil {
			if err != nil {
				if resp != nil {
					_ = resp.Body.Close()
				}
				return nil, fmt.Errorf("call enrichment provider: %w", errors.Join(err, ErrEnrichmentCall))
			}
			_ = resp.Body.Close()
			return nil, fmt.Errorf("enrichment provider returned %s: %w", resp.Status, ErrEnrichmentCall)
		}

		if resp != nil {
			_ = resp.Body.Close()
		}
		if err := waitForRetry(ctx, backoff); err != nil {
			return nil, fmt.Errorf("wait to retry enrichment provider: %w", errors.Join(err, ErrEnrichmentCall))
		}
		backoff = nextBackoff(backoff, provider.Retry.MaxBackoff)
	}

	return nil, ErrEnrichmentCall
}

func waitForRetry(ctx context.Context, backoff time.Duration) error {
	if backoff <= 0 {
		return nil
	}
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextBackoff(current, maximum time.Duration) time.Duration {
	if current <= 0 {
		return 0
	}
	if maximum > 0 && current >= maximum/2 {
		return maximum
	}
	return current * 2
}

func validateEndpoint(endpoint string) (string, error) {
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("endpoint must use http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("endpoint must include a host")
	}
	return parsed.String(), nil
}
