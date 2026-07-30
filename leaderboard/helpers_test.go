package leaderboard_test

import (
	"context"
	"os"

	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/testing"
)

var testRedisServer *testing.RedisServer
var faultyRedisServer *testing.RedisServer

// Creates an empty context (shortcut for context.Background())
func NewEmptyCtx() context.Context {
	return context.Background()
}

func GetDefaultRedis() (*database.Redis, error) {
	if os.Getenv("PODIUM_TEST_REDIS_EXTERNAL") == "true" {
		config, err := testing.GetDefaultConfig("../config/test.yaml")
		if err != nil {
			return nil, err
		}
		return database.NewRedisDatabase(database.RedisOptions{
			ClusterEnabled: config.GetBool("redis.cluster.enabled"),
			Addrs:          config.GetStringSlice("redis.addrs"),
			Host:           config.GetString("redis.host"),
			Port:           config.GetInt("redis.port"),
			Password:       config.GetString("redis.password"),
			DB:             config.GetInt("redis.db"),
		}), nil
	}

	if testRedisServer == nil {
		server, err := testing.StartRedis()
		if err != nil {
			return nil, err
		}
		testRedisServer = server
	}

	return database.NewRedisDatabase(database.RedisOptions{
		Host: testRedisServer.Host,
		Port: testRedisServer.Port,
	}), nil
}

func GetFaultyRedis() (*database.Redis, error) {
	if faultyRedisServer == nil {
		server, err := testing.StartRedis()
		if err != nil {
			return nil, err
		}
		server.SetError("ERR injected Redis failure")
		faultyRedisServer = server
	}
	return database.NewRedisDatabase(database.RedisOptions{
		Host: faultyRedisServer.Host,
		Port: faultyRedisServer.Port,
	}), nil
}
