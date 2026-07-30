//go:build rediscluster

package leaderboard_test

import (
	"context"
	"os"
	"strings"
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

func TestRedisClusterTopology(t *testing.T) {
	addresses := strings.FieldsFunc(os.Getenv("PODIUM_REDIS_ADDRS"), func(r rune) bool {
		return r == ',' || r == ' '
	})
	if len(addresses) == 0 {
		t.Fatal("PODIUM_REDIS_ADDRS must contain at least one Redis Cluster node")
	}

	client := goredis.NewClusterClient(&goredis.ClusterOptions{
		Addrs:    addresses,
		Password: os.Getenv("PODIUM_REDIS_PASSWORD"),
	})
	t.Cleanup(func() {
		_ = client.Close()
	})

	slots, err := client.ClusterSlots(context.Background()).Result()
	if err != nil {
		t.Fatalf("query Redis Cluster slots: %v", err)
	}

	primaries := make(map[string]struct{})
	coveredSlots := 0
	for _, slot := range slots {
		if len(slot.Nodes) == 0 {
			t.Fatalf("slot range %d-%d has no primary node", slot.Start, slot.End)
		}
		primaries[slot.Nodes[0].Addr] = struct{}{}
		coveredSlots += int(slot.End-slot.Start) + 1
	}
	if len(primaries) < 3 {
		t.Fatalf("expected at least three Redis Cluster primaries, got %d", len(primaries))
	}
	if coveredSlots != 16384 {
		t.Fatalf("expected all 16384 Redis Cluster slots, got %d", coveredSlots)
	}
}
