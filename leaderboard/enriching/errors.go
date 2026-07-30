package enriching

import "errors"

var (
	// ErrEnrichmentCall is returned when the enrichment provider call fails.
	ErrEnrichmentCall = errors.New("the enrichment provider returned an error")

	// ErrEnrichmentInternal is returned when the enrichment fails for an internal reason.
	ErrEnrichmentInternal = errors.New("could not perform enrichment")
)

type ProviderError struct {
	Strict bool
	Err    error
}

func (e *ProviderError) Error() string {
	return e.Err.Error()
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

func IsStrictFailure(err error) bool {
	var providerErr *ProviderError
	return errors.As(err, &providerErr) && providerErr.Strict
}
