// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/TeneficGames/podium/config"
	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/observability"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// ExpirationResult is the struct that represents the result of an expiration job
type ExpirationResult struct {
	DeletedMembers int
	DeletedSet     bool
	Set            string
}

func (r *ExpirationResult) String() string {
	return fmt.Sprintf("(DeletedMembers: %d, DeletedSet: %t, Set: %s)", r.DeletedMembers, r.DeletedSet, r.Set)
}

// ExpirationWorker is the struct that represents the scores expirer worker
type ExpirationWorker struct {
	Config                  *viper.Viper
	Database                database.Expiration
	ConfigPath              string
	ExpirationCheckInterval time.Duration
	ExpirationLimitPerRun   int
	stop                    chan struct{}
	stopOnce                sync.Once
	runDuration             metric.Float64Histogram
	expiredMembers          metric.Int64Counter
	runErrors               metric.Int64Counter
}

// GetExpirationWorker returns a new scores expirer worker
func GetExpirationWorker(configPath string) (*ExpirationWorker, error) {
	worker := &ExpirationWorker{
		ConfigPath: configPath,
	}

	err := worker.loadConfiguration()
	if err != nil {
		return nil, err
	}

	err = worker.configure()
	if err != nil {
		return nil, err
	}

	return worker, nil
}

// NewExpirationWorker returns a new scores expirer worker with already loaded configuration.
func NewExpirationWorker(host string, port int, password string, db int,
	expirationCheckInterval time.Duration, expirationLimitPerRun int) (*ExpirationWorker, error) {

	worker := &ExpirationWorker{
		ConfigPath: "../config/default.yaml",
		Config:     viper.New(),
	}
	worker.Config.Set("redis.host", host)
	worker.Config.Set("redis.port", port)
	worker.Config.Set("redis.password", password)
	worker.Config.Set("redis.db", db)
	worker.Config.Set("worker.expirationCheckInterval", expirationCheckInterval)
	worker.Config.Set("worker.expirationLimitPerRun", expirationLimitPerRun)

	err := worker.configure()
	if err != nil {
		return nil, err
	}

	return worker, nil
}

func (w *ExpirationWorker) loadConfiguration() error {
	config, err := config.GetDefaultConfig(w.ConfigPath)
	if err != nil {
		return err
	}
	w.Config = config
	return nil

}

func (w *ExpirationWorker) configure() error {
	w.setConfigurationDefaults()
	w.ExpirationCheckInterval = w.Config.GetDuration("worker.expirationCheckInterval")
	w.ExpirationLimitPerRun = w.Config.GetInt("worker.expirationLimitPerRun")
	w.stop = make(chan struct{})

	database := database.NewRedisDatabase(database.RedisOptions{
		ClusterEnabled: w.Config.GetBool("redis.cluster.enabled"),
		Addrs:          w.Config.GetStringSlice("redis.addrs"),
		Host:           w.Config.GetString("redis.host"),
		Port:           w.Config.GetInt("redis.port"),
		Password:       w.Config.GetString("redis.password"),
		DB:             w.Config.GetInt("redis.db"),
	})
	w.Database = database
	return w.configureInstrumentation()
}

func (w *ExpirationWorker) configureInstrumentation() error {
	meter := otel.Meter("github.com/TeneficGames/podium/worker")
	var err error
	w.runDuration, err = meter.Float64Histogram(
		"podium.expiration.run.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of a leaderboard expiration run"),
	)
	if err != nil {
		return fmt.Errorf("create expiration run duration histogram: %w", err)
	}
	w.expiredMembers, err = meter.Int64Counter(
		"podium.expiration.members",
		metric.WithDescription("Members removed by the expiration worker"),
	)
	if err != nil {
		return fmt.Errorf("create expired members counter: %w", err)
	}
	w.runErrors, err = meter.Int64Counter(
		"podium.expiration.errors",
		metric.WithDescription("Errors encountered by the expiration worker"),
	)
	if err != nil {
		return fmt.Errorf("create expiration errors counter: %w", err)
	}
	return nil
}

func (w *ExpirationWorker) setConfigurationDefaults() {
	w.Config.SetDefault("redis.clusterEnabled", "false")
	w.Config.SetDefault("redis.addrs", "")
	w.Config.SetDefault("redis.host", "localhost")
	w.Config.SetDefault("redis.port", "6379")
	w.Config.SetDefault("redis.password", "")
	w.Config.SetDefault("redis.db", 0)
	w.Config.SetDefault("redis.maxPoolSize", 20)
	w.Config.SetDefault("worker.expirationCheckInterval", "60s")
	w.Config.SetDefault("worker.expirationLimitPerRun", "1000")
}

// Stop finish expiration worker execution
func (w *ExpirationWorker) Stop() {
	w.stopOnce.Do(func() {
		close(w.stop)
	})
}

// Run execute a new worker
func (w *ExpirationWorker) Run(resultsChan chan<- []*ExpirationResult, errChan chan<- error) {
	shouldEnd := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.runWorker(shouldEnd, resultsChan, errChan)
	}()

	select {
	case <-sigChan:
	case <-w.stop:
	}

	signal.Stop(sigChan)
	close(shouldEnd)
	<-done
}

func (w *ExpirationWorker) runWorker(shouldEnd <-chan struct{}, resultsChan chan<- []*ExpirationResult, errChan chan<- error) {
	ticker := time.NewTicker(w.ExpirationCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-shouldEnd:
			return
		case <-ticker.C:
			w.expireMembers(resultsChan, errChan)
		}
	}

}

func (w *ExpirationWorker) expireMembers(resultsChan chan<- []*ExpirationResult, errChan chan<- error) {
	start := time.Now()
	ctx, span := otel.Tracer("github.com/TeneficGames/podium/worker").Start(
		context.Background(),
		"expiration.run",
	)
	defer func() {
		w.runDuration.Record(ctx, time.Since(start).Seconds())
		span.End()
	}()

	leaderboardExpirations, err := w.Database.GetExpirationLeaderboards(ctx)
	if err != nil {
		w.recordError(ctx, span, err)
		errChan <- err
		return
	}

	result := []*ExpirationResult{}
	for _, leaderboard := range leaderboardExpirations {
		expirationResult, err := w.expireMembersFromLeaderboard(ctx, leaderboard)
		if err != nil {
			w.recordError(ctx, span, err)
			errChan <- err
			return
		}

		result = append(result, expirationResult)
		w.expiredMembers.Add(ctx, int64(expirationResult.DeletedMembers))
	}
	span.SetAttributes(attribute.Int("expiration.leaderboards", len(result)))
	resultsChan <- result
}

func (w *ExpirationWorker) expireMembersFromLeaderboard(ctx context.Context, leaderboard string) (*ExpirationResult, error) {
	ctx, span := otel.Tracer("github.com/TeneficGames/podium/worker").Start(
		ctx,
		"expiration.leaderboard",
		trace.WithAttributes(attribute.String("leaderboard.id", leaderboard)),
	)
	defer span.End()

	members, err := w.Database.GetMembersToExpire(ctx, leaderboard, w.ExpirationLimitPerRun, time.Now().UTC())
	if err != nil {
		var noMembersErr *database.LeaderboardWithoutMemberToExpireError
		if errors.As(err, &noMembersErr) {
			err = w.Database.RemoveLeaderboardFromExpireList(ctx, leaderboard)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return nil, err
			}

			return &ExpirationResult{
				DeletedMembers: 0,
				DeletedSet:     true,
				Set:            leaderboard,
			}, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if len(members) == 0 {
		return &ExpirationResult{
			DeletedMembers: 0,
			DeletedSet:     false,
			Set:            leaderboard,
		}, nil
	}

	err = w.Database.ExpireMembers(ctx, leaderboard, members)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.Int("expiration.deleted_members", len(members)))

	return &ExpirationResult{
		DeletedMembers: len(members),
		DeletedSet:     false,
		Set:            leaderboard,
	}, nil
}

func (w *ExpirationWorker) recordError(ctx context.Context, span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	w.runErrors.Add(ctx, 1)
	observability.CaptureException(ctx, err, map[string]string{"source": "expiration-worker"}, nil)
}
