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
	"context"
	"fmt"
	"os"

	"github.com/TeneficGames/podium/api"
	"github.com/TeneficGames/podium/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var host string
var httpPort int
var grpcPort int
var debug, quiet bool

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "starts the podium API server",
	Long: `Starts podium server with the specified arguments. You can use
	environment variables to override configuration keys.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ll := zap.InfoLevel
		if debug {
			ll = zap.DebugLevel
		}
		if quiet {
			ll = zap.WarnLevel
		}

		logger := log.CreateLoggerWithLevel(ll, log.LoggerOptions{WriteSyncer: os.Stdout})
		logger = logger.With(
			zap.String("source", "app"),
		)

		defer func() {
			_ = logger.Sync()
		}()

		app, err := api.New(
			host,
			httpPort,
			grpcPort,
			ConfigFile,
			debug,
			logger,
		)

		if err != nil {
			return fmt.Errorf("create podium application: %w", err)
		}

		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		err = app.Start(ctx)
		if err != nil {
			return fmt.Errorf("start podium application: %w", err)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(startCmd)

	startCmd.Flags().StringVarP(&host, "bind", "b", "0.0.0.0", "Host to bind podium to")
	startCmd.Flags().IntVarP(&httpPort, "http_port", "p", 8880, "HTTP Port to bind podium to")
	startCmd.Flags().IntVarP(&grpcPort, "grpc_port", "g", 8881, "GRPC Port to bind podium to")
	startCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Debug mode (log=debug)")
	startCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode (log=warn)")
}
