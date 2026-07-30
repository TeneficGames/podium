package enriching

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	podium_enrichment_v1 "github.com/TeneficGames/podium/leaderboard/enriching/proto/enrichment/v1"
	"github.com/TeneficGames/podium/leaderboard/model"
)

func TestEnrichWithHTTPProvider(t *testing.T) {
	var received podium_enrichment_v1.EnrichMembersRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/members/enrich" {
			t.Errorf("expected configured endpoint path, got %q", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("expected provider authorization header, got %q", got)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
		}
		_ = json.NewEncoder(writer).Encode(&podium_enrichment_v1.EnrichMembersResponse{
			Members: []*podium_enrichment_v1.Member{{
				Id:       "member-1",
				Metadata: map[string]string{"display_name": "Alice"},
			}},
		})
	}))
	defer server.Close()

	enricher := NewEnricher(WithProviders(map[string]Provider{
		"tenant": {
			Endpoint: server.URL + "/v1/members/enrich",
			Headers:  map[string]string{"Authorization": "Bearer token"},
		},
	}))
	members := []*model.Member{{PublicID: "member-1", Score: 1200, Rank: 3}}

	result, err := enricher.Enrich(context.Background(), "tenant", "leaderboard", members)
	if err != nil {
		t.Fatalf("enrich members: %v", err)
	}
	if received.TenantId != "tenant" || received.LeaderboardId != "leaderboard" {
		t.Fatalf("unexpected request context: tenant=%q leaderboard=%q",
			received.TenantId, received.LeaderboardId)
	}
	if len(received.Members) != 1 || received.Members[0].Id != "member-1" ||
		received.Members[0].Score != 1200 || received.Members[0].Rank != 3 {
		t.Fatalf("unexpected request members: %#v", received.Members)
	}
	if got := result[0].Metadata["display_name"]; got != "Alice" {
		t.Fatalf("expected enriched display name, got %q", got)
	}
}

func TestEnrichSkipsUnconfiguredProvider(t *testing.T) {
	members := []*model.Member{{PublicID: "member"}}
	result, err := NewEnricher().Enrich(context.Background(), "tenant", "leaderboard", members)
	if err != nil {
		t.Fatalf("enrich members: %v", err)
	}
	if result[0] != members[0] {
		t.Fatal("expected members to be returned unchanged")
	}
}

func TestEnrichReturnsMembersAndErrorWhenProviderFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "failed", http.StatusBadGateway)
	}))
	defer server.Close()

	enricher := NewEnricher(WithProviders(map[string]Provider{
		"tenant": {Endpoint: server.URL},
	}))
	members := []*model.Member{{PublicID: "member"}}

	result, err := enricher.Enrich(context.Background(), "tenant", "leaderboard", members)
	if err == nil {
		t.Fatal("expected provider error")
	}
	if result[0] != members[0] {
		t.Fatal("expected original members to be returned with the error")
	}
	if IsStrictFailure(err) {
		t.Fatal("expected the default failure mode to be best effort")
	}
}

func TestEnrichMarksStrictProviderFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "failed", http.StatusBadGateway)
	}))
	defer server.Close()

	enricher := NewEnricher(WithProviders(map[string]Provider{
		"tenant": {
			Endpoint: server.URL,
			Mode:     FailureModeStrict,
		},
	}))

	_, err := enricher.Enrich(
		context.Background(),
		"tenant",
		"leaderboard",
		[]*model.Member{{PublicID: "member"}},
	)
	if err == nil || !IsStrictFailure(err) {
		t.Fatalf("expected a strict provider failure, got %v", err)
	}
}

func TestEnrichRejectsInvalidProviderResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("invalid"))
	}))
	defer server.Close()

	enricher := NewEnricher(WithProviders(map[string]Provider{
		"tenant": {Endpoint: server.URL},
	}))

	if _, err := enricher.Enrich(
		context.Background(),
		"tenant",
		"leaderboard",
		[]*model.Member{{PublicID: "member"}},
	); err == nil {
		t.Fatal("expected invalid provider response error")
	}
}

func TestValidateEndpoint(t *testing.T) {
	for _, endpoint := range []string{"localhost:8080/enrich", "ftp://example.com/enrich", "/enrich"} {
		if _, err := validateEndpoint(endpoint); err == nil {
			t.Errorf("expected endpoint %q to be rejected", endpoint)
		}
	}

	const endpoint = "https://example.com/v1/members/enrich"
	if got, err := validateEndpoint(endpoint); err != nil || got != endpoint {
		t.Fatalf("expected endpoint to be accepted, got %q, %v", got, err)
	}
}

func TestProviderRetriesRetryableResponsesUntilSuccess(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		switch attempts {
		case 1:
			http.Error(writer, "unavailable", http.StatusServiceUnavailable)
		case 2:
			http.Error(writer, "rate limited", http.StatusTooManyRequests)
		default:
			_ = json.NewEncoder(writer).Encode(&podium_enrichment_v1.EnrichMembersResponse{
				Members: []*podium_enrichment_v1.Member{{
					Id:       "member",
					Metadata: map[string]string{"retried": "true"},
				}},
			})
		}
	}))
	defer server.Close()

	enricher := NewEnricher(WithProviders(map[string]Provider{
		"tenant": {
			Endpoint: server.URL,
			Retry: RetryConfig{
				MaxAttempts: 3,
			},
		},
	}))

	result, err := enricher.Enrich(
		context.Background(),
		"tenant",
		"leaderboard",
		[]*model.Member{{PublicID: "member"}},
	)
	if err != nil {
		t.Fatalf("enrich after retries: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if result[0].Metadata["retried"] != "true" {
		t.Fatal("expected metadata from the successful retry")
	}
}

func TestProviderDoesNotRetryNonRetryableResponse(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		http.Error(writer, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	enricher := NewEnricher(WithProviders(map[string]Provider{
		"tenant": {
			Endpoint: server.URL,
			Retry: RetryConfig{
				MaxAttempts: 3,
			},
		},
	}))

	_, err := enricher.Enrich(
		context.Background(),
		"tenant",
		"leaderboard",
		[]*model.Member{{PublicID: "member"}},
	)
	if err == nil {
		t.Fatal("expected provider error")
	}
	if attempts != 1 {
		t.Fatalf("expected one attempt for a non-retryable response, got %d", attempts)
	}
}

func TestProviderStopsAtMaximumAttempts(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	enricher := NewEnricher(WithProviders(map[string]Provider{
		"tenant": {
			Endpoint: server.URL,
			Retry: RetryConfig{
				MaxAttempts: 3,
			},
		},
	}))

	_, err := enricher.Enrich(
		context.Background(),
		"tenant",
		"leaderboard",
		[]*model.Member{{PublicID: "member"}},
	)
	if err == nil {
		t.Fatal("expected provider error")
	}
	if attempts != 3 {
		t.Fatalf("expected retries to stop after 3 attempts, got %d", attempts)
	}
}

func TestProviderRetriesTransportErrors(t *testing.T) {
	attempts := 0
	enricher := NewEnricher(WithProviders(map[string]Provider{
		"tenant": {
			Endpoint: "https://example.com/enrich",
			Retry: RetryConfig{
				MaxAttempts: 2,
			},
		},
	})).(*enricherImpl)
	enricher.client.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("connection reset")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"members":[]}`)),
		}, nil
	})

	if _, err := enricher.Enrich(
		context.Background(),
		"tenant",
		"leaderboard",
		[]*model.Member{{PublicID: "member"}},
	); err != nil {
		t.Fatalf("enrich after transport retry: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected transport error to be retried, got %d attempts", attempts)
	}
}

func TestWaitForRetryHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := waitForRetry(ctx, time.Minute)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("expected canceled backoff to return immediately, took %s", elapsed)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestNextBackoff(t *testing.T) {
	if got := nextBackoff(50*time.Millisecond, 500*time.Millisecond); got != 100*time.Millisecond {
		t.Fatalf("expected doubled backoff, got %s", got)
	}
	if got := nextBackoff(400*time.Millisecond, 500*time.Millisecond); got != 500*time.Millisecond {
		t.Fatalf("expected capped backoff, got %s", got)
	}
}
