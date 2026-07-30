package enriching

import (
	"errors"
	"time"

	"go.uber.org/zap"
)

type (
	enrichmentConfig struct {
		providers      map[string]Provider
		requestTimeout time.Duration
	}

	Provider struct {
		Endpoint string
		Headers  map[string]string
		Mode     FailureMode
		Retry    RetryConfig
	}

	RetryConfig struct {
		MaxAttempts    int
		InitialBackoff time.Duration
		MaxBackoff     time.Duration
	}
)

type FailureMode string

const (
	FailureModeBestEffort FailureMode = "best_effort"
	FailureModeStrict     FailureMode = "strict"
)

func (p Provider) Validate() error {
	if _, err := validateEndpoint(p.Endpoint); err != nil {
		return err
	}
	if p.Mode != "" && p.Mode != FailureModeBestEffort && p.Mode != FailureModeStrict {
		return errors.New("mode must be best_effort or strict")
	}
	if p.Retry.MaxAttempts < 0 {
		return errors.New("retry max_attempts must not be negative")
	}
	if p.Retry.InitialBackoff < 0 || p.Retry.MaxBackoff < 0 {
		return errors.New("retry backoff must not be negative")
	}
	if p.Retry.MaxBackoff > 0 && p.Retry.InitialBackoff > p.Retry.MaxBackoff {
		return errors.New("retry initial_backoff must not exceed max_backoff")
	}
	return nil
}

func newDefaultEnrichConfig() enrichmentConfig {
	return enrichmentConfig{
		providers:      map[string]Provider{},
		requestTimeout: 500 * time.Millisecond,
	}
}

type EnricherOptions func(*enricherImpl)

// WithProviders sets the HTTP enrichment provider for each tenant.
func WithProviders(providers map[string]Provider) EnricherOptions {
	return func(impl *enricherImpl) {
		impl.config.providers = providers
	}
}

// WithRequestTimeout sets the timeout for enrichment provider calls.
func WithRequestTimeout(timeout time.Duration) EnricherOptions {
	return func(impl *enricherImpl) {
		if timeout > 0 {
			impl.config.requestTimeout = timeout
		}
	}
}

// WithLogger sets the logger.
func WithLogger(logger *zap.Logger) EnricherOptions {
	return func(impl *enricherImpl) {
		impl.logger = logger.With(zap.String("source", "enricher"))
	}
}
