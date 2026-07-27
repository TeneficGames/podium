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

func TestDecodeHook(t *testing.T) {
	if DecodeHook() == nil {
		t.Fatal("expected a decoder option")
	}
}

func TestStringToMapStringHook(t *testing.T) {
	hook := StringToMapStringHookFunc().(func(reflect.Type, reflect.Type, interface{}) (interface{}, error))
	stringType := reflect.TypeOf("")
	stringMapType := reflect.TypeOf(map[string]string{})

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
			to:    stringMapType,
			input: `{"game-a":"https://example.com"}`,
			want:  map[string]string{"game-a": "https://example.com"},
		},
		{
			name:  "empty string",
			from:  stringType,
			to:    stringMapType,
			input: "",
			want:  map[string]string{},
		},
		{
			name:    "invalid JSON",
			from:    stringType,
			to:      stringMapType,
			input:   "{",
			want:    map[string]string{},
			wantErr: true,
		},
		{
			name:  "non-string input",
			from:  reflect.TypeOf(42),
			to:    stringMapType,
			input: 42,
			want:  42,
		},
		{
			name:  "non-string map values",
			from:  stringType,
			to:    reflect.TypeOf(map[string]int{}),
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

func TestStringToMapBoolHook(t *testing.T) {
	hook := StringToMapBoolHookFunc().(func(reflect.Type, reflect.Type, interface{}) (interface{}, error))
	stringType := reflect.TypeOf("")
	boolMapType := reflect.TypeOf(map[string]bool{})

	tests := []struct {
		name    string
		from    reflect.Type
		to      reflect.Type
		input   interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name:  "valid values",
			from:  stringType,
			to:    boolMapType,
			input: `{"game-a":"true","game-b":"false","invalid":"maybe"}`,
			want:  map[string]bool{"game-a": true, "game-b": false},
		},
		{
			name:  "empty string",
			from:  stringType,
			to:    boolMapType,
			input: "",
			want:  map[string]bool{},
		},
		{
			name:    "invalid JSON",
			from:    stringType,
			to:      boolMapType,
			input:   "{",
			want:    map[string]bool{},
			wantErr: true,
		},
		{
			name:  "non-string input",
			from:  reflect.TypeOf(42),
			to:    boolMapType,
			input: 42,
			want:  42,
		},
		{
			name:  "non-bool map values",
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
