package leaderboard_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/service"
	podiumtesting "github.com/TeneficGames/podium/leaderboard/testing"
)

func TestIdempotentIncrementAcrossInstances(t *testing.T) {
	opts := database.RedisOptions{Host: "localhost", Port: 6379, Password: os.Getenv("PODIUM_REDIS_PASSWORD")}
	if os.Getenv("PODIUM_TEST_REDIS_EXTERNAL") == "true" {
		opts.ClusterEnabled = os.Getenv("PODIUM_REDIS_CLUSTER_ENABLED") == "true"
		opts.Addrs = strings.FieldsFunc(os.Getenv("PODIUM_REDIS_ADDRS"), func(r rune) bool { return r == ',' || r == ' ' })
		if host := os.Getenv("PODIUM_REDIS_HOST"); host != "" {
			opts.Host = host
		}
		if port := os.Getenv("PODIUM_REDIS_PORT"); port != "" {
			var err error
			opts.Port, err = strconv.Atoi(port)
			if err != nil {
				t.Fatal(err)
			}
		}
	} else {
		server, err := podiumtesting.StartRedis()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(server.Close)
		opts.Host = server.Host
		opts.Port = server.Port
		opts.Password = ""
	}
	first := database.NewRedisDatabase(opts)
	second := database.NewRedisDatabase(opts)
	t.Cleanup(func() {
		_ = first.Client.(interface{ Close() error }).Close()
		_ = second.Client.(interface{ Close() error }).Close()
	})
	ctx := context.Background()
	board := fmt.Sprintf("idempotency-instances-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if err := first.RemoveLeaderboard(ctx, board); err != nil {
			t.Error(err)
		}
	})
	one, two := service.NewService(first), service.NewService(second)
	result, err := one.IncrementMemberScoreOnce(ctx, board, "alice", 100, "60", "match")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := two.IncrementMemberScore(ctx, board, "alice", 1, ""); err != nil {
		t.Fatal(err)
	}
	replay, err := two.IncrementMemberScoreOnce(ctx, board, "alice", 100, "60", "match")
	if err != nil || !reflect.DeepEqual(result, replay) {
		t.Fatalf("replicas did not share original response: %+v / %+v, %v", result, replay, err)
	}
	members, err := first.GetMembers(ctx, board, "desc", true, "alice")
	if err != nil || members[0].Score != 101 || members[0].TTL.Unix() != int64(result.ExpireAt) {
		t.Fatalf("retry mutated score/TTL: %+v, %v", members, err)
	}
}
