//go:build rediscompat

package leaderboard_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

var redisVersionPattern = regexp.MustCompile(`(?m)^redis_version:(\d+\.\d+)(?:\.\d+)?\r?$`)

func TestRedisCompatibilityVersion(t *testing.T) {
	expectedVersion := os.Getenv("PODIUM_TEST_REDIS_VERSION")
	if expectedVersion == "" {
		t.Fatal("PODIUM_TEST_REDIS_VERSION must name the Redis minor version under test")
	}

	client := goredis.NewClient(&goredis.Options{
		Addr: fmt.Sprintf(
			"%s:%s",
			envOrDefault("PODIUM_REDIS_HOST", "localhost"),
			envOrDefault("PODIUM_REDIS_PORT", "6379"),
		),
		Password: os.Getenv("PODIUM_REDIS_PASSWORD"),
	})
	t.Cleanup(func() {
		_ = client.Close()
	})

	info, err := client.Info(context.Background(), "server").Result()
	if err != nil {
		t.Fatalf("query Redis server version: %v", err)
	}
	matches := redisVersionPattern.FindStringSubmatch(info)
	if len(matches) != 2 {
		t.Fatalf("Redis INFO response did not contain a version: %q", info)
	}
	if matches[1] != expectedVersion {
		t.Fatalf("connected to Redis %s, expected %s", matches[1], expectedVersion)
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
