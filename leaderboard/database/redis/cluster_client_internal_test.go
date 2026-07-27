// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func TestClusterClientAgainstStandaloneRedis(t *testing.T) {
	const (
		address = "localhost:6379"
		key     = "podium:cluster-client:test"
	)

	goRedisClient := goredis.NewClusterClient(&goredis.ClusterOptions{
		Addrs: []string{address},
		ClusterSlots: func(context.Context) ([]goredis.ClusterSlot, error) {
			return []goredis.ClusterSlot{{
				Start: 0,
				End:   16383,
				Nodes: []goredis.ClusterNode{{Addr: address}},
			}}, nil
		},
	})
	t.Cleanup(func() {
		_ = goRedisClient.Del(context.Background(), key, key+":ttl", key+"-writes").Err()
		_ = goRedisClient.Close()
	})

	client := &clusterClient{ClusterClient: goRedisClient}
	ctx := context.Background()

	if pong, err := client.Ping(ctx); err != nil || pong != "PONG" {
		t.Fatalf("expected PONG, got %q, %v", pong, err)
	}
	if err := client.Del(ctx, key); err != nil {
		t.Fatalf("delete missing key: %v", err)
	}
	if err := client.Exists(ctx, key); !isKeyNotFound(err) {
		t.Fatalf("expected key-not-found error, got %v", err)
	}
	if err := goRedisClient.Set(ctx, key, "value", 0).Err(); err != nil {
		t.Fatalf("seed string: %v", err)
	}
	if err := client.Exists(ctx, key); err != nil {
		t.Fatalf("check existing key: %v", err)
	}
	if _, err := client.TTL(ctx, key); !isTTLNotFound(err) {
		t.Fatalf("expected TTL-not-found error, got %v", err)
	}
	expiration := time.Now().Add(time.Minute)
	if err := client.ExpireAt(ctx, key, expiration); err != nil {
		t.Fatalf("set expiration: %v", err)
	}
	if ttl, err := client.TTL(ctx, key); err != nil || ttl <= 0 {
		t.Fatalf("expected positive TTL, got %s, %v", ttl, err)
	}
	if err := client.Del(ctx, key); err != nil {
		t.Fatalf("delete string: %v", err)
	}
	if err := client.ExpireAt(ctx, key, expiration); !isKeyNotFound(err) {
		t.Fatalf("expected missing expiration key, got %v", err)
	}
	if _, err := client.TTL(ctx, key); !isKeyNotFound(err) {
		t.Fatalf("expected missing TTL key, got %v", err)
	}

	if err := client.SAdd(ctx, key, "member-1"); err != nil {
		t.Fatalf("add set member: %v", err)
	}
	if err := client.SAdd(ctx, key, "member-2"); err != nil {
		t.Fatalf("add second set member: %v", err)
	}
	members, err := client.SMembers(ctx, key)
	if err != nil || len(members) != 2 {
		t.Fatalf("expected two set members, got %v, %v", members, err)
	}
	if err := client.SRem(ctx, key, "member-1", "member-2"); err != nil {
		t.Fatalf("remove set members: %v", err)
	}

	sortedMembers := []*Member{
		{Member: "member-1", Score: 1},
		{Member: "member-2", Score: 2},
	}
	if err := client.ZAdd(ctx, key, sortedMembers...); err != nil {
		t.Fatalf("add sorted-set members: %v", err)
	}
	if count, err := client.ZCard(ctx, key); err != nil || count != 2 {
		t.Fatalf("expected two sorted-set members, got %d, %v", count, err)
	}
	if err := client.ZIncrBy(ctx, key, "member-1", 2); err != nil {
		t.Fatalf("increment score: %v", err)
	}
	if score, err := client.ZScore(ctx, key, "member-1"); err != nil || score != 3 {
		t.Fatalf("expected score 3, got %v, %v", score, err)
	}
	if rank, err := client.ZRank(ctx, key, "member-1"); err != nil || rank != 1 {
		t.Fatalf("expected ascending rank 1, got %d, %v", rank, err)
	}
	if rank, err := client.ZRevRank(ctx, key, "member-1"); err != nil || rank != 0 {
		t.Fatalf("expected descending rank 0, got %d, %v", rank, err)
	}
	if err := client.ZAdd(ctx, key+":ttl", &Member{Member: "member-1", Score: 10000}); err != nil {
		t.Fatalf("add member TTL: %v", err)
	}
	rankedMembers, err := client.ZMembers(ctx, key, "desc", true, "member-1", "missing", "member-2")
	if err != nil {
		t.Fatalf("get members: %v", err)
	}
	if len(rankedMembers) != 3 ||
		rankedMembers[0] == nil || rankedMembers[0].Score != 3 || rankedMembers[0].Rank != 0 || rankedMembers[0].TTL.Unix() != 10000 ||
		rankedMembers[1] != nil ||
		rankedMembers[2] == nil || rankedMembers[2].Score != 2 || rankedMembers[2].Rank != 1 {
		t.Fatalf("unexpected members: %#v", rankedMembers)
	}

	ranks, err := client.ZAddAndRanks(
		ctx,
		key+"-writes",
		"desc",
		&Member{Member: "member-1", Score: 5},
		&Member{Member: "member-2", Score: 4},
	)
	if err != nil || len(ranks) != 2 || ranks[0] != 0 || ranks[1] != 1 {
		t.Fatalf("unexpected write ranks: %v, %v", ranks, err)
	}

	incremented, err := client.ZIncrByAndRank(ctx, key+"-writes", "member-2", "desc", 2)
	if err != nil || incremented.Score != 6 || incremented.Rank != 0 {
		t.Fatalf("unexpected incremented member: %#v, %v", incremented, err)
	}

	ascending, err := client.ZRange(ctx, key, 0, -1)
	if err != nil || len(ascending) != 2 || ascending[1].Member != "member-1" {
		t.Fatalf("unexpected ascending members: %#v, %v", ascending, err)
	}
	descending, err := client.ZRevRange(ctx, key, 0, -1)
	if err != nil || len(descending) != 2 || descending[0].Member != "member-1" {
		t.Fatalf("unexpected descending members: %#v, %v", descending, err)
	}
	byScore, err := client.ZRangeByScore(ctx, key, "2", "3", 0, 10)
	if err != nil || len(byScore) != 2 {
		t.Fatalf("unexpected score range: %#v, %v", byScore, err)
	}
	reverseByScore, err := client.ZRevRangeByScore(ctx, key, "2", "3", 0, 10)
	if err != nil || len(reverseByScore) != 2 || reverseByScore[0] != "member-1" {
		t.Fatalf("unexpected reverse score range: %#v, %v", reverseByScore, err)
	}

	if _, err := client.ZRank(ctx, key, "missing"); !isMemberNotFound(err) {
		t.Fatalf("expected missing rank member, got %v", err)
	}
	if _, err := client.ZRevRank(ctx, key, "missing"); !isMemberNotFound(err) {
		t.Fatalf("expected missing reverse-rank member, got %v", err)
	}
	if _, err := client.ZScore(ctx, key, "missing"); !isMemberNotFound(err) {
		t.Fatalf("expected missing score member, got %v", err)
	}
	if err := client.ZRem(ctx, key, "member-1", "member-2"); err != nil {
		t.Fatalf("remove sorted-set members: %v", err)
	}
	if _, err := client.ZCard(ctx, key); !isKeyNotFound(err) {
		t.Fatalf("expected missing sorted set, got %v", err)
	}
}

func TestNewClusterClient(t *testing.T) {
	client := NewClusterClient(ClusterOptions{Addrs: []string{"localhost:6379"}})
	cluster, ok := client.(*clusterClient)
	if !ok {
		t.Fatalf("expected cluster client, got %T", client)
	}
	if err := cluster.Close(); err != nil {
		t.Fatalf("close cluster client: %v", err)
	}
}

func TestClusterClientWrapsConnectionErrors(t *testing.T) {
	client := NewClusterClient(ClusterOptions{Addrs: []string{"localhost:1"}})
	cluster := client.(*clusterClient)
	t.Cleanup(func() {
		_ = cluster.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		call func() error
	}{
		{name: "delete", call: func() error { return client.Del(ctx, "key") }},
		{name: "exists", call: func() error { return client.Exists(ctx, "key") }},
		{name: "expire", call: func() error { return client.ExpireAt(ctx, "key", time.Now()) }},
		{name: "ping", call: func() error { _, err := client.Ping(ctx); return err }},
		{name: "set add", call: func() error { return client.SAdd(ctx, "key", "member") }},
		{name: "set members", call: func() error { _, err := client.SMembers(ctx, "key"); return err }},
		{name: "set remove", call: func() error { return client.SRem(ctx, "key", "member") }},
		{name: "TTL", call: func() error { _, err := client.TTL(ctx, "key"); return err }},
		{name: "sorted add", call: func() error {
			return client.ZAdd(ctx, "key", &Member{Member: "member", Score: 1})
		}},
		{name: "sorted card", call: func() error { _, err := client.ZCard(ctx, "key"); return err }},
		{name: "sorted increment", call: func() error {
			return client.ZIncrBy(ctx, "key", "member", 1)
		}},
		{name: "sorted range", call: func() error {
			_, err := client.ZRange(ctx, "key", 0, -1)
			return err
		}},
		{name: "sorted range by score", call: func() error {
			_, err := client.ZRangeByScore(ctx, "key", "-inf", "+inf", 0, 10)
			return err
		}},
		{name: "sorted rank", call: func() error { _, err := client.ZRank(ctx, "key", "member"); return err }},
		{name: "sorted remove", call: func() error { return client.ZRem(ctx, "key", "member") }},
		{name: "sorted reverse range", call: func() error {
			_, err := client.ZRevRange(ctx, "key", 0, -1)
			return err
		}},
		{name: "sorted reverse range by score", call: func() error {
			_, err := client.ZRevRangeByScore(ctx, "key", "-inf", "+inf", 0, 10)
			return err
		}},
		{name: "sorted reverse rank", call: func() error {
			_, err := client.ZRevRank(ctx, "key", "member")
			return err
		}},
		{name: "sorted score", call: func() error {
			_, err := client.ZScore(ctx, "key", "member")
			return err
		}},
		{name: "sorted members", call: func() error {
			_, err := client.(*clusterClient).ZMembers(ctx, "key", "desc", false, "member")
			return err
		}},
		{name: "sorted add and ranks", call: func() error {
			_, err := client.(*clusterClient).ZAddAndRanks(
				ctx,
				"key",
				"desc",
				&Member{Member: "member", Score: 1},
			)
			return err
		}},
		{name: "sorted increment and rank", call: func() error {
			_, err := client.(*clusterClient).ZIncrByAndRank(ctx, "key", "member", "desc", 1)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var generalError *GeneralError
			if err := tt.call(); !errors.As(err, &generalError) {
				t.Fatalf("expected general Redis error, got %v", err)
			}
		})
	}
}

func isKeyNotFound(err error) bool {
	var target *KeyNotFoundError
	return errors.As(err, &target)
}

func isTTLNotFound(err error) bool {
	var target *TTLNotFoundError
	return errors.As(err, &target)
}

func isMemberNotFound(err error) bool {
	var target *MemberNotFoundError
	return errors.As(err, &target)
}
