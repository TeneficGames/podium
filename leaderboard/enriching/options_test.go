package enriching

import (
	"context"
	"testing"
	"time"

	"github.com/TeneficGames/podium/leaderboard/model"
	"go.uber.org/zap"
)

func TestEnricherOptions(t *testing.T) {
	const timeout = 2 * time.Second
	providers := map[string]Provider{"tenant": {Endpoint: "https://example.com/enrich"}}
	enricher := NewEnricher(
		WithProviders(providers),
		WithRequestTimeout(timeout),
		WithLogger(zap.NewNop()),
	).(*enricherImpl)

	if enricher.config.requestTimeout != timeout || enricher.client.Timeout != timeout {
		t.Fatalf("expected request timeout %s, got config=%s client=%s",
			timeout, enricher.config.requestTimeout, enricher.client.Timeout)
	}
	if enricher.config.providers["tenant"].Endpoint != providers["tenant"].Endpoint {
		t.Fatal("expected providers to be configured")
	}
	if enricher.logger == nil {
		t.Fatal("expected logger to be configured")
	}
}

func TestEnrichReturnsEmptyMembersWithoutCallingAProvider(t *testing.T) {
	enricher := NewEnricher()
	members := []*model.Member{}

	result, err := enricher.Enrich(context.Background(), "tenant", "leaderboard", members)
	if err != nil {
		t.Fatalf("enrich empty members: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected no members, got %d", len(result))
	}
}

func TestZeroRequestTimeoutKeepsDefault(t *testing.T) {
	enricher := NewEnricher(WithRequestTimeout(0)).(*enricherImpl)
	if enricher.client.Timeout != 500*time.Millisecond {
		t.Fatalf("expected default request timeout, got %s", enricher.client.Timeout)
	}
}

func TestProviderValidation(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
	}{
		{name: "invalid endpoint", provider: Provider{Endpoint: "localhost:8080"}},
		{name: "invalid mode", provider: Provider{Endpoint: "https://example.com", Mode: "unknown"}},
		{
			name: "negative attempts",
			provider: Provider{
				Endpoint: "https://example.com",
				Retry:    RetryConfig{MaxAttempts: -1},
			},
		},
		{
			name: "initial backoff exceeds maximum",
			provider: Provider{
				Endpoint: "https://example.com",
				Retry: RetryConfig{
					InitialBackoff: time.Second,
					MaxBackoff:     time.Millisecond,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.provider.Validate(); err == nil {
				t.Fatal("expected provider validation error")
			}
		})
	}

	for _, mode := range []FailureMode{"", FailureModeBestEffort, FailureModeStrict} {
		provider := Provider{Endpoint: "https://example.com", Mode: mode}
		if err := provider.Validate(); err != nil {
			t.Fatalf("expected mode %q to be valid: %v", mode, err)
		}
	}
}

func TestEnrichSkipsEmptyProviderEndpoint(t *testing.T) {
	members := []*model.Member{{PublicID: "member"}}
	enricher := NewEnricher(WithProviders(map[string]Provider{"tenant": {}}))

	result, err := enricher.Enrich(context.Background(), "tenant", "leaderboard", members)
	if err != nil {
		t.Fatalf("enrich with empty provider endpoint: %v", err)
	}
	if len(result) != 1 || result[0] != members[0] {
		t.Fatalf("expected members to be returned unchanged, got %#v", result)
	}
}
