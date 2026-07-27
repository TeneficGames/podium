// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestExecute(t *testing.T) {
	called := false
	command := &cobra.Command{
		Use: "podium",
		Run: func(*cobra.Command, []string) {
			called = true
		},
	}

	Execute(command)

	if !called {
		t.Fatal("expected command to run")
	}
}

func TestInitConfig(t *testing.T) {
	previousConfigFile := ConfigFile
	ConfigFile = ""
	viper.Reset()
	t.Setenv("PODIUM_API_PORT", "8880")
	t.Cleanup(func() {
		ConfigFile = previousConfigFile
		viper.Reset()
	})

	initConfig()

	if got := viper.GetInt("api.port"); got != 8880 {
		t.Fatalf("expected API port 8880 from the environment, got %d", got)
	}
}

func TestInitConfigUsesExplicitFile(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "podium.yaml")
	if err := os.WriteFile(configFile, []byte("api:\n  port: 8881\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	previousConfigFile := ConfigFile
	ConfigFile = configFile
	viper.Reset()
	t.Cleanup(func() {
		ConfigFile = previousConfigFile
		viper.Reset()
	})

	initConfig()

	if got := viper.GetInt("api.port"); got != 8881 {
		t.Fatalf("expected API port 8881 from %s, got %d", configFile, got)
	}
	if got := viper.ConfigFileUsed(); got != configFile {
		t.Fatalf("expected config file %s, got %s", configFile, got)
	}
}

func TestVersionCommand(t *testing.T) {
	versionCmd.Run(versionCmd, nil)
}

func TestStartCommandReturnsConfigurationError(t *testing.T) {
	previousConfigFile := ConfigFile
	previousDebug := debug
	previousQuiet := quiet
	ConfigFile = filepath.Join(t.TempDir(), "missing.yaml")
	debug = true
	quiet = true
	t.Cleanup(func() {
		ConfigFile = previousConfigFile
		debug = previousDebug
		quiet = previousQuiet
	})

	if err := startCmd.RunE(startCmd, nil); err == nil {
		t.Fatal("expected invalid configuration to fail")
	}
}

func TestWorkerCommandReturnsConfigurationError(t *testing.T) {
	previousConfigFile := ConfigFile
	previousDebug := debug
	previousQuiet := quiet
	ConfigFile = filepath.Join(t.TempDir(), "missing.yaml")
	debug = true
	quiet = true
	t.Cleanup(func() {
		ConfigFile = previousConfigFile
		debug = previousDebug
		quiet = previousQuiet
	})

	if err := workerCmd.RunE(workerCmd, nil); err == nil {
		t.Fatal("expected invalid configuration to fail")
	}
}
