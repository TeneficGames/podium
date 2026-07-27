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
	"os"
	"time"

	"github.com/TeneficGames/podium/log"
	"github.com/TeneficGames/podium/observability"
	"github.com/TeneficGames/podium/worker"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// workerCmd represents the worker command
var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "starts the podium scores expirer worker",
	Long: `starts the podium worker that expires scores with the specified arguments. you can use environment variables to override
	configuration keys`,
	Run: func(cmd *cobra.Command, args []string) {
		ll := zap.InfoLevel
		if debug {
			ll = zap.DebugLevel
		}
		if quiet {
			ll = zap.WarnLevel
		}
		logger := log.CreateLoggerWithLevel(ll, log.LoggerOptions{WriteSyncer: os.Stdout})
		logger = logger.With(
			zap.String("source", "worker"),
		)

		defer func() {
			_ = logger.Sync()
		}()

		logger.Info("Starting podium score expirer worker...")

		telemetry, err := observability.New(cmd.Context(), "podium-worker")
		if err != nil {
			logger.Fatal("Could not configure observability.", zap.Error(err))
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := telemetry.Shutdown(ctx); err != nil {
				logger.Error("Could not shut down observability.", zap.Error(err))
			}
		}()

		w, err := worker.GetExpirationWorker(ConfigFile)

		if err != nil {
			logger.Fatal("Could not get podium worker.", zap.Error(err))
		}

		expirationsChan := make(chan []*worker.ExpirationResult)
		errChan := make(chan error)

		go func() {
			for {
				select {
				case expirations := <-expirationsChan:
					logger.Debug("expiration results", zap.Any("result", expirations))
				case err := <-errChan:
					logger.Error("error from worker", zap.Error(err))
				}
			}
		}()

		w.Run(expirationsChan, errChan)
	},
}

func init() {
	workerCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Debug mode (log=debug)")
	workerCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode (log=warn)")
	RootCmd.AddCommand(workerCmd)
}
