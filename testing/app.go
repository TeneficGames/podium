package testing

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/TeneficGames/podium/api"
	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/database/redis"
	"github.com/TeneficGames/podium/leaderboard/service"
	leaderboardtesting "github.com/TeneficGames/podium/leaderboard/testing"
	"github.com/TeneficGames/podium/log"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var serverInitialized map[string]bool = map[string]bool{}
var defaultApp *api.App
var defaultFaultyRedisApp *api.App
var testRedisServer *leaderboardtesting.RedisServer
var faultyTestRedisServer *leaderboardtesting.RedisServer

// StartTestRedis starts the shared in-process Redis server used by API tests.
func StartTestRedis() error {
	if testRedisServer != nil {
		return nil
	}
	server, err := leaderboardtesting.StartRedis()
	if err != nil {
		return err
	}
	for key, value := range map[string]string{
		"PODIUM_REDIS_CLUSTER_ENABLED": "false",
		"PODIUM_REDIS_HOST":            server.Host,
		"PODIUM_REDIS_PORT":            fmt.Sprint(server.Port),
		"PODIUM_REDIS_PASSWORD":        "",
		"PODIUM_REDIS_DB":              "0",
	} {
		if err := os.Setenv(key, value); err != nil {
			server.Close()
			return err
		}
	}
	testRedisServer = server
	return nil
}

// StopTestRedis stops the shared API test Redis server.
func StopTestRedis() {
	if testRedisServer != nil {
		testRedisServer.Close()
		testRedisServer = nil
	}
	if faultyTestRedisServer != nil {
		faultyTestRedisServer.Close()
		faultyTestRedisServer = nil
	}
}

// GetDefaultTestApp returns a testing app
func GetDefaultTestApp() *api.App {
	if defaultApp != nil {
		return defaultApp
	}
	if err := StartTestRedis(); err != nil {
		panic(fmt.Sprintf("Could not start test Redis: %s\n", err.Error()))
	}

	logger := log.CreateLoggerWithLevel(zap.FatalLevel, log.LoggerOptions{WriteSyncer: os.Stdout, RemoveTimestamp: true})
	app, err := api.New("127.0.0.1", 0, 0, "../config/test.yaml", true, logger)
	if err != nil {
		panic(fmt.Sprintf("Could not get app: %s\n", err.Error()))
	}
	defaultApp = app
	return app
}

// ShutdownDefaultTestApp turn off default test app
func ShutdownDefaultTestApp() {
	if defaultApp != nil {
		defaultApp.GracefullStop()
	}
}

// InitializeTestServer starts the test server
func InitializeTestServer(app *api.App) {
	if client == nil {
		transport = &http.Transport{DisableKeepAlives: true}
		client = &http.Client{Transport: transport}
	}

	if !serverInitialized[app.ID.String()] {
		serverInitialized[app.ID.String()] = true
		go func() {
			err := app.Start(context.Background())
			if err != nil {
				panic(err)
			}
		}()
		err := app.WaitForReady(500 * time.Millisecond)
		Expect(err).NotTo(HaveOccurred())
	}
}

// GetDefaultTestAppWithFaultyRedis returns a new podium API Application bound to 0.0.0.0:8890 for test but with a failing Redis
func GetDefaultTestAppWithFaultyRedis() *api.App {
	if defaultFaultyRedisApp != nil {
		return defaultFaultyRedisApp
	}
	Expect(StartTestRedis()).To(Succeed())
	var err error
	faultyTestRedisServer, err = leaderboardtesting.StartRedis()
	Expect(err).NotTo(HaveOccurred())
	faultyTestRedisServer.SetError("ERR injected Redis failure")

	logger := log.CreateLoggerWithLevel(zapcore.DebugLevel, log.LoggerOptions{WriteSyncer: os.Stdout, RemoveTimestamp: true})
	app, err := api.New("127.0.0.1", 8082, 8083, "../config/test.yaml", false, logger)
	Expect(err).NotTo(HaveOccurred())

	leaderboard := service.NewService(database.NewRedisDatabase(database.RedisOptions{
		Host: faultyTestRedisServer.Host,
		Port: faultyTestRedisServer.Port,
	}))
	app.Leaderboards = leaderboard

	defaultFaultyRedisApp = app
	return app
}

// ShutdownDefaultTestAppWithFaltyRedis turn off default test app
func ShutdownDefaultTestAppWithFaltyRedis() {
	if defaultFaultyRedisApp != nil {
		defaultFaultyRedisApp.GracefullStop()
	}
}

// GetTestingRedis creates a redis instance based on the default app configuration
func GetTestingRedis(app *api.App) (redis.Client, error) {
	shouldRunOnCluster := app.Config.GetBool("redis.cluster.enabled")
	password := app.Config.GetString("redis.password")
	if shouldRunOnCluster {
		addrs := app.Config.GetStringSlice("redis.addrs")
		return redis.NewClusterClient(redis.ClusterOptions{
			Addrs:    addrs,
			Password: password,
		}), nil
	}

	return redis.NewStandaloneClient(redis.StandaloneOptions{
		Host:     app.Config.GetString("redis.host"),
		Port:     app.Config.GetInt("redis.port"),
		Password: password,
		DB:       app.Config.GetInt("redis.db"),
	}), nil
}
