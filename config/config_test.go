// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestGetDefaultConfig(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "podium.yaml")
	if err := os.WriteFile(configFile, []byte("api:\n  port: 8080\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, err := GetDefaultConfig(configFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got := config.GetInt("api.port"); got != 8080 {
		t.Fatalf("expected API port 8080, got %d", got)
	}
}

func TestGetDefaultConfigReturnsReadError(t *testing.T) {
	if _, err := GetDefaultConfig(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("expected missing config file to return an error")
	}
}

func TestEnrichmentProviderConfigFromYAML(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "podium.yaml")
	contents := `enrichment:
  request_timeout: 750ms
  providers:
    game:
      endpoint: https://example.com/enrich
      mode: strict
      headers:
        Authorization: Bearer token
      retry:
        max_attempts: 3
        initial_backoff: 50ms
        max_backoff: 500ms
`
	if err := os.WriteFile(configFile, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	viperConfig, err := GetDefaultConfig(configFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	var parsed PodiumConfig
	if err := viperConfig.Unmarshal(&parsed, DecodeHook()); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	provider := parsed.Enrichment.Providers["game"]
	if provider.Endpoint != "https://example.com/enrich" || provider.Mode != "strict" {
		t.Fatalf("unexpected provider: %#v", provider)
	}
	if provider.Headers["authorization"] != "Bearer token" {
		t.Fatalf("unexpected headers: %#v", provider.Headers)
	}
	if provider.Retry.MaxAttempts != 3 ||
		provider.Retry.InitialBackoff != 50*time.Millisecond ||
		provider.Retry.MaxBackoff != 500*time.Millisecond {
		t.Fatalf("unexpected retry config: %#v", provider.Retry)
	}
	if parsed.Enrichment.RequestTimeout != 750*time.Millisecond {
		t.Fatalf("unexpected request timeout: %s", parsed.Enrichment.RequestTimeout)
	}
}

func TestDecodeHook(t *testing.T) {
	if DecodeHook() == nil {
		t.Fatal("expected a decoder option")
	}
}

func TestStringToEnrichmentProvidersHook(t *testing.T) {
	hook := StringToEnrichmentProvidersHookFunc().(func(reflect.Type, reflect.Type, interface{}) (interface{}, error))
	stringType := reflect.TypeOf("")
	providersType := reflect.TypeOf(map[string]EnrichmentProviderConfig{})

	tests := []struct {
		name    string
		from    reflect.Type
		to      reflect.Type
		input   interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name:  "valid JSON",
			from:  stringType,
			to:    providersType,
			input: `{"game-a":{"endpoint":"https://example.com/enrich","headers":{"Authorization":"Bearer token"},"mode":"strict","retry":{"max_attempts":3,"initial_backoff":"50ms","max_backoff":"500ms"}}}`,
			want: map[string]EnrichmentProviderConfig{
				"game-a": {
					Endpoint: "https://example.com/enrich",
					Headers:  map[string]string{"Authorization": "Bearer token"},
					Mode:     "strict",
					Retry: RetryConfig{
						MaxAttempts:    3,
						InitialBackoff: 50 * time.Millisecond,
						MaxBackoff:     500 * time.Millisecond,
					},
				},
			},
		},
		{
			name:  "empty string",
			from:  stringType,
			to:    providersType,
			input: "",
			want:  map[string]EnrichmentProviderConfig{},
		},
		{
			name:    "invalid JSON",
			from:    stringType,
			to:      providersType,
			input:   "{",
			want:    map[string]EnrichmentProviderConfig{},
			wantErr: true,
		},
		{
			name:    "invalid backoff",
			from:    stringType,
			to:      providersType,
			input:   `{"game-a":{"retry":{"initial_backoff":"invalid"}}}`,
			want:    map[string]EnrichmentProviderConfig{},
			wantErr: true,
		},
		{
			name:  "non-string input",
			from:  reflect.TypeOf(42),
			to:    providersType,
			input: 42,
			want:  42,
		},
		{
			name:  "different map type",
			from:  stringType,
			to:    reflect.TypeOf(map[string]string{}),
			input: "unchanged",
			want:  "unchanged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hook(tt.from, tt.to, tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("expected %#v, got %#v", tt.want, got)
			}
		})
	}
}
