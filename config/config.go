package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// GetDefaultConfig configure viper to use the config file
func GetDefaultConfig(configFile string) (*viper.Viper, error) {
	config := viper.New()
	config.SetConfigFile(configFile)
	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	config.SetEnvPrefix("podium")
	config.AddConfigPath("$HOME")
	config.AutomaticEnv()

	err := config.ReadInConfig()
	if err != nil {
		return nil, err
	}
	return config, nil
}

type (
	PodiumConfig struct {
		Enrichment EnrichmentConfig
	}

	EnrichmentConfig struct {
		// Providers contains the HTTP enrichment provider configuration for each tenant.
		Providers map[string]EnrichmentProviderConfig `mapstructure:"providers"`

		// RequestTimeout is the timeout for enrichment provider calls.
		RequestTimeout time.Duration `mapstructure:"request_timeout"`

		Cache Cache `mapstructure:"cache"`
	}

	EnrichmentProviderConfig struct {
		Endpoint string            `mapstructure:"endpoint" json:"endpoint"`
		Headers  map[string]string `mapstructure:"headers" json:"headers"`
		Mode     string            `mapstructure:"mode" json:"mode"`
		Retry    RetryConfig       `mapstructure:"retry" json:"retry"`
	}

	RetryConfig struct {
		MaxAttempts    int           `mapstructure:"max_attempts" json:"max_attempts"`
		InitialBackoff time.Duration `mapstructure:"initial_backoff" json:"initial_backoff"`
		MaxBackoff     time.Duration `mapstructure:"max_backoff" json:"max_backoff"`
	}

	Cache struct {
		// Add is the address for the cache.
		Addr string `mapstructure:"addr"`

		// Password is the password for the cache.
		Password string `mapstructure:"password"`

		// TTL is the time to live for the cached data.
		TTL time.Duration `mapstructure:"ttl"`
	}
)

func DecodeHook() viper.DecoderConfigOption {
	decodeHook := mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		StringToEnrichmentProvidersHookFunc(),
	)

	return viper.DecodeHook(decodeHook)

}

func StringToEnrichmentProvidersHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{},
	) (interface{}, error) {
		if f.Kind() != reflect.String || t.Kind() != reflect.Map {
			return data, nil
		}

		providersType := reflect.TypeOf(map[string]EnrichmentProviderConfig{})
		if t != providersType {
			return data, nil
		}

		raw := data.(string)
		if raw == "" {
			return map[string]EnrichmentProviderConfig{}, nil
		}

		wireProviders := map[string]struct {
			Endpoint string            `json:"endpoint"`
			Headers  map[string]string `json:"headers"`
			Mode     string            `json:"mode"`
			Retry    struct {
				MaxAttempts    int    `json:"max_attempts"`
				InitialBackoff string `json:"initial_backoff"`
				MaxBackoff     string `json:"max_backoff"`
			} `json:"retry"`
		}{}
		if err := json.Unmarshal([]byte(raw), &wireProviders); err != nil {
			return map[string]EnrichmentProviderConfig{}, err
		}

		providers := make(map[string]EnrichmentProviderConfig, len(wireProviders))
		for tenantID, provider := range wireProviders {
			initialBackoff, err := parseOptionalDuration(provider.Retry.InitialBackoff)
			if err != nil {
				return map[string]EnrichmentProviderConfig{}, fmt.Errorf(
					"parse initial backoff for tenant %q: %w", tenantID, err,
				)
			}
			maxBackoff, err := parseOptionalDuration(provider.Retry.MaxBackoff)
			if err != nil {
				return map[string]EnrichmentProviderConfig{}, fmt.Errorf(
					"parse max backoff for tenant %q: %w", tenantID, err,
				)
			}
			providers[tenantID] = EnrichmentProviderConfig{
				Endpoint: provider.Endpoint,
				Headers:  provider.Headers,
				Mode:     provider.Mode,
				Retry: RetryConfig{
					MaxAttempts:    provider.Retry.MaxAttempts,
					InitialBackoff: initialBackoff,
					MaxBackoff:     maxBackoff,
				},
			}
		}
		return providers, nil
	}
}

func parseOptionalDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	return time.ParseDuration(value)
}
