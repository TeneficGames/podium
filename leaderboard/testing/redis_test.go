package testing_test

import (
	"context"
	"testing"

	podiumtesting "github.com/TeneficGames/podium/leaderboard/testing"
	goredis "github.com/redis/go-redis/v9"
)

func TestStartRedis(t *testing.T) {
	server, err := podiumtesting.StartRedis()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	if server.Host == "" || server.Port == 0 {
		t.Fatalf("invalid test Redis address: %s:%d", server.Host, server.Port)
	}

	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	pong, err := client.Ping(context.Background()).Result()
	if err != nil {
		t.Fatal(err)
	}
	if pong != "PONG" {
		t.Fatalf("expected PONG, got %q", pong)
	}
}
