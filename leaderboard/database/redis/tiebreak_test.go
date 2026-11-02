package redis_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	podiumredis "github.com/TeneficGames/podium/leaderboard/database/redis"
	"github.com/TeneficGames/podium/leaderboard/expiration"
	"github.com/TeneficGames/podium/leaderboard/model"
	podiumtesting "github.com/TeneficGames/podium/leaderboard/testing"
	goredis "github.com/redis/go-redis/v9"
)

const maxSafeRedisInteger = float64(9_007_199_254_740_992)

func TestTieBreakStoreOrdering(t *testing.T) {
	store, _, keys := newTieBreakStore(t)
	ctx := context.Background()

	assertRanks(t, store, keys, []string{"alice", "bob"}, []int64{0, 1})
	assertMemberRanks(t, store, keys, "asc", map[string]int64{"alice": 0, "bob": 1})

	ranks, err := store.UpsertMembersWithTieBreak(ctx, keys, "desc",
		&podiumredis.Member{Member: "alice", Score: 100},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranks) != 1 || ranks[0] != 0 {
		t.Fatalf("same-score submission changed alice's rank: %v", ranks)
	}

	if _, err := store.UpsertMembersWithTieBreak(ctx, keys, "desc",
		&podiumredis.Member{Member: "bob", Score: 110},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertMembersWithTieBreak(ctx, keys, "desc",
		&podiumredis.Member{Member: "bob", Score: 100},
	); err != nil {
		t.Fatal(err)
	}
	assertMemberRanks(t, store, keys, "desc", map[string]int64{"alice": 0, "bob": 1})

	if err := store.ExpireMembersWithTieBreak(ctx, keys, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertMembersWithTieBreak(ctx, keys, "desc",
		&podiumredis.Member{Member: "alice", Score: 100},
	); err != nil {
		t.Fatal(err)
	}
	assertMemberRanks(t, store, keys, "desc", map[string]int64{"bob": 0, "alice": 1})
	assertMemberRanks(t, store, keys, "asc", map[string]int64{"bob": 0, "alice": 1})
}

func TestTieBreakStoreBulkReturnsFinalRanks(t *testing.T) {
	store, _, keys := newTieBreakStore(t)
	ranks, err := store.UpsertMembersWithTieBreak(context.Background(), keys, "desc",
		&podiumredis.Member{Member: "lower", Score: 90},
		&podiumredis.Member{Member: "higher", Score: 100},
	)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(ranks) != "[1 0]" {
		t.Fatalf("expected final ranks [1 0], got %v", ranks)
	}
}

func TestTieBreakStoreIncrement(t *testing.T) {
	store, client, keys := newTieBreakStore(t)
	ctx := context.Background()

	if _, err := store.UpsertMembersWithTieBreak(ctx, keys, "desc",
		&podiumredis.Member{Member: "alice", Score: 100},
		&podiumredis.Member{Member: "bob", Score: 90},
	); err != nil {
		t.Fatal(err)
	}

	bob, err := store.IncrementWithTieBreak(ctx, keys, "bob", "desc", 10)
	if err != nil {
		t.Fatal(err)
	}
	if bob.Score != 100 || bob.Rank != 1 {
		t.Fatalf("bob reached the tie later: got score=%v rank=%d", bob.Score, bob.Rank)
	}

	alice, err := store.IncrementWithTieBreak(ctx, keys, "alice", "desc", 0)
	if err != nil {
		t.Fatal(err)
	}
	if alice.Score != 100 || alice.Rank != 0 {
		t.Fatalf("zero increment changed alice's position: %+v", alice)
	}

	const increments = 128
	var wg sync.WaitGroup
	errs := make(chan error, increments)
	for range increments {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.IncrementWithTieBreak(ctx, keys, "concurrent", "desc", 1)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	member, err := store.GetMembersWithTieBreak(ctx, keys, "desc", false, "concurrent")
	if err != nil {
		t.Fatal(err)
	}
	if member[0] == nil || member[0].Score != increments {
		t.Fatalf("concurrent increments lost updates: %+v", member[0])
	}
	for _, key := range []string{keys.Scores, keys.ScoresAsc} {
		count, err := client.ZCard(ctx, key).Result()
		if err != nil {
			t.Fatal(err)
		}
		if count != 3 {
			t.Fatalf("stale internal members were retained in %s: got %d entries", key, count)
		}
	}
}

func TestTieBreakStoreConcurrentArrivals(t *testing.T) {
	store, _, keys := newTieBreakStore(t)
	ctx := context.Background()
	const membersCount = 128

	var wg sync.WaitGroup
	errs := make(chan error, membersCount)
	for i := range membersCount {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.UpsertMembersWithTieBreak(ctx, keys, "desc",
				&podiumredis.Member{Member: fmt.Sprintf("member-%03d", i), Score: 100},
			)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	ids := make([]string, membersCount)
	for i := range membersCount {
		ids[i] = fmt.Sprintf("member-%03d", i)
	}
	members, err := store.GetMembersWithTieBreak(ctx, keys, "desc", false, ids...)
	if err != nil {
		t.Fatal(err)
	}
	seenRanks := make([]bool, membersCount)
	for _, member := range members {
		if member == nil {
			t.Fatal("concurrent arrival was lost")
		}
		if member.Rank < 0 || member.Rank >= membersCount || seenRanks[member.Rank] {
			t.Fatalf("rank is not unique: %d", member.Rank)
		}
		seenRanks[member.Rank] = true
	}
}

func TestTieBreakStoreBoundariesAndMissingMembers(t *testing.T) {
	store, _, keys := newTieBreakStore(t)
	ctx := context.Background()

	if _, err := store.UpsertMembersWithTieBreak(ctx, keys, "desc",
		&podiumredis.Member{Member: "maximum", Score: maxSafeRedisInteger},
		&podiumredis.Member{Member: "zero", Score: 0},
		&podiumredis.Member{Member: "minimum", Score: -maxSafeRedisInteger},
	); err != nil {
		t.Fatal(err)
	}
	assertMemberRanks(t, store, keys, "desc", map[string]int64{
		"maximum": 0,
		"zero":    1,
		"minimum": 2,
	})

	members, err := store.GetMembersWithTieBreak(ctx, keys, "desc", false, "missing", "zero")
	if err != nil {
		t.Fatal(err)
	}
	if members[0] != nil || members[1] == nil || members[1].Score != 0 {
		t.Fatalf("missing-member alignment was not preserved: %+v", members)
	}
	if _, err := store.GetRankWithTieBreak(ctx, keys, "missing", "desc"); err == nil {
		t.Fatal("expected a missing-member rank error")
	}

	for _, score := range []float64{maxSafeRedisInteger + 2, -(maxSafeRedisInteger + 2)} {
		if _, err := store.UpsertMembersWithTieBreak(ctx, keys, "desc",
			&podiumredis.Member{Member: "out-of-range", Score: score},
		); err == nil {
			t.Fatalf("expected score %v to be rejected", score)
		}
	}
	if _, err := store.IncrementWithTieBreak(ctx, keys, "maximum", "desc", 1); err == nil {
		t.Fatal("expected an increment beyond the exact score range to be rejected")
	}
}

func TestTieBreakStoreTTLExpirationAndDeletion(t *testing.T) {
	store, rawClient, keys := newTieBreakStore(t)
	ctx := context.Background()
	expiresAt := time.Now().Add(10 * time.Minute).Truncate(time.Second)

	if _, err := store.UpsertMembersWithTieBreak(ctx, keys, "desc",
		&podiumredis.Member{Member: "alice", Score: 100},
		&podiumredis.Member{Member: "bob", Score: 90},
	); err != nil {
		t.Fatal(err)
	}
	if err := store.SetMembersTTLWithTieBreak(ctx, keys,
		&podiumredis.Member{Member: "alice", TTL: expiresAt},
		&podiumredis.Member{Member: "bob", TTL: expiresAt.Add(time.Minute)},
	); err != nil {
		t.Fatal(err)
	}

	members, err := store.GetMembersWithTieBreak(ctx, keys, "desc", true, "alice", "bob")
	if err != nil {
		t.Fatal(err)
	}
	if !members[0].TTL.Equal(expiresAt) || !members[1].TTL.Equal(expiresAt.Add(time.Minute)) {
		t.Fatalf("unexpected member TTLs: %v, %v", members[0].TTL, members[1].TTL)
	}

	if err := store.ExpireMembersWithTieBreak(ctx, keys, "alice"); err != nil {
		t.Fatal(err)
	}
	members, err = store.GetMembersWithTieBreak(ctx, keys, "desc", true, "alice", "bob")
	if err != nil {
		t.Fatal(err)
	}
	if members[0] != nil || members[1] == nil {
		t.Fatalf("member removal did not clean score, mapping, and TTL: %+v", members)
	}

	if err := store.ExpireTieBreakKeysAt(ctx, keys, expiresAt); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{keys.Scores, keys.ScoresAsc, keys.Members, keys.Sequence, keys.TTL} {
		ttl, err := rawClient.PTTL(ctx, key).Result()
		if err != nil {
			t.Fatal(err)
		}
		if ttl <= 0 {
			t.Fatalf("key %s did not receive leaderboard expiration", key)
		}
	}

	if err := store.DeleteLeaderboard(ctx, keys); err != nil {
		t.Fatal(err)
	}
	existing, err := rawClient.Exists(
		ctx,
		keys.Scores,
		keys.ScoresAsc,
		keys.Members,
		keys.Sequence,
		keys.TTL,
	).Result()
	if err != nil {
		t.Fatal(err)
	}
	if existing != 0 {
		t.Fatalf("leaderboard deletion left %d keys", existing)
	}
}

func TestTieBreakStoreRejectsExpirationForMissingLeaderboard(t *testing.T) {
	store, _, keys := newTieBreakStore(t)

	err := store.ExpireTieBreakKeysAt(context.Background(), keys, time.Now().Add(time.Hour))
	if err == nil {
		t.Fatal("expected a missing leaderboard error")
	}
}

func TestTieBreakStoreRejectsExhaustedSequence(t *testing.T) {
	store, rawClient, keys := newTieBreakStore(t)
	ctx := context.Background()
	if _, err := store.UpsertMembersWithTieBreak(ctx, keys, "desc",
		&podiumredis.Member{Member: "existing", Score: 100},
	); err != nil {
		t.Fatal(err)
	}
	if err := rawClient.Set(ctx, keys.Sequence, "0", 0).Err(); err != nil {
		t.Fatal(err)
	}

	_, err := store.UpsertMembersWithTieBreak(ctx, keys, "desc",
		&podiumredis.Member{Member: "existing", Score: 100},
		&podiumredis.Member{Member: "new", Score: 100},
	)
	if err == nil || !strings.Contains(err.Error(), "sequence exhausted") {
		t.Fatalf("expected sequence exhaustion error, got %v", err)
	}
	count, err := rawClient.ZCard(ctx, keys.Scores).Result()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("exhausted update partially mutated the leaderboard: got %d members", count)
	}
	members, err := store.GetMembersWithTieBreak(ctx, keys, "desc", false, "existing", "new")
	if err != nil {
		t.Fatal(err)
	}
	if members[0] == nil || members[1] != nil {
		t.Fatalf("exhausted update changed leaderboard contents: %+v", members)
	}
}

func newTieBreakStore(t *testing.T) (podiumredis.TieBreakStore, *goredis.Client, podiumredis.TieBreakKeys) {
	t.Helper()
	server, err := podiumtesting.StartRedis()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	client := podiumredis.NewStandaloneClient(podiumredis.StandaloneOptions{
		Host: server.Host, Port: server.Port,
	})
	store, ok := client.(podiumredis.TieBreakStore)
	if !ok {
		t.Fatal("standalone Redis client does not implement TieBreakStore")
	}
	rawClient := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	tag := "{tiebreak-test-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "}"
	keys := podiumredis.TieBreakKeys{
		Scores:    tag + ":scores",
		ScoresAsc: tag + ":scores-asc",
		Members:   tag + ":members",
		Sequence:  tag + ":sequence",
		TTL:       tag + ":ttl",
	}
	t.Cleanup(func() {
		_ = store.DeleteLeaderboard(context.Background(), keys)
		_ = rawClient.Close()
	})
	return store, rawClient, keys
}

func assertRanks(
	t *testing.T,
	store podiumredis.TieBreakStore,
	keys podiumredis.TieBreakKeys,
	ids []string,
	expected []int64,
) {
	t.Helper()
	members := make([]*podiumredis.Member, len(ids))
	for i, id := range ids {
		members[i] = &podiumredis.Member{Member: id, Score: 100}
	}
	ranks, err := store.UpsertMembersWithTieBreak(context.Background(), keys, "desc", members...)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(ranks) != fmt.Sprint(expected) {
		t.Fatalf("expected ranks %v, got %v", expected, ranks)
	}
}

func assertMemberRanks(
	t *testing.T,
	store podiumredis.TieBreakStore,
	keys podiumredis.TieBreakKeys,
	order string,
	expected map[string]int64,
) {
	t.Helper()
	for member, expectedRank := range expected {
		rank, err := store.GetRankWithTieBreak(context.Background(), keys, member, order)
		if err != nil {
			t.Fatal(err)
		}
		if rank != expectedRank {
			t.Fatalf("%s rank: expected %d, got %d", member, expectedRank, rank)
		}
	}
}

func TestIncrementOnceStoreResponseAndFailures(t *testing.T) {
	store, client, keys := newTieBreakStore(t)
	ctx := context.Background()
	for _, tc := range []struct {
		key       string
		increment float64
	}{{"", 1}, {"event", 0}, {"event", math.NaN()}} {
		if _, err := podiumredis.IncrementOnceWithTieBreak(ctx, client, keys, "alice", tc.increment, tc.key, 0, -1); err == nil {
			t.Fatal("invalid increment request accepted")
		}
	}
	original, err := podiumredis.IncrementOnceWithTieBreak(ctx, client, keys, "alice", 10, "event", 60, -1)
	if err != nil {
		t.Fatal(err)
	}
	if original.Score != 10 || original.Rank != 0 || original.TTL.IsZero() {
		t.Fatalf("unexpected original response: %+v", original)
	}
	if _, err := store.UpsertMembersWithTieBreak(ctx, keys, "desc", &podiumredis.Member{Member: "bob", Score: 100}); err != nil {
		t.Fatal(err)
	}
	replay, err := podiumredis.IncrementOnceWithTieBreak(ctx, client, keys, "alice", 10, "event", 60, -1)
	if err != nil || !reflect.DeepEqual(original, replay) {
		t.Fatalf("original response changed: %+v, %v", replay, err)
	}
	_, err = podiumredis.IncrementOnceWithTieBreak(ctx, client, keys, "alice", 11, "event", 60, -1)
	if !errors.Is(err, model.ErrIdempotencyConflict) {
		t.Fatalf("expected conflict: %v", err)
	}
	plain, err := podiumredis.IncrementOnceWithTieBreak(ctx, client, keys, "alice", 1, "plain", 0, -1)
	if err != nil || plain.Score != 11 || !plain.TTL.IsZero() {
		t.Fatalf("unexpected no-TTL response: %+v, %v", plain, err)
	}
	_, err = podiumredis.IncrementOnceWithTieBreak(ctx, client, keys, "alice", 1, "expired", 0, -2)
	var expired *expiration.LeaderboardExpiredError
	if !errors.As(err, &expired) {
		t.Fatalf("expected expired leaderboard: %v", err)
	}
}

func TestCorruptIdempotencyResponseDoesNotRepeatIncrement(t *testing.T) {
	store, client, keys := newTieBreakStore(t)
	ctx := context.Background()
	original, err := podiumredis.IncrementOnceWithTieBreak(ctx, client, keys, "alice", 10, "event", 60, -1)
	if err != nil {
		t.Fatal(err)
	}
	cacheKeys, err := client.Keys(ctx, keys.Scores+":idempotency:*").Result()
	if err != nil || len(cacheKeys) != 1 {
		t.Fatalf("expected one replay record: %v, %v", cacheKeys, err)
	}
	cached, err := client.Get(ctx, cacheKeys[0]).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		field int
	}{
		{"incomplete response", -1}, {"invalid score", 1}, {"invalid rank", 2}, {"invalid expiry", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var record []any
			if err := json.Unmarshal(cached, &record); err != nil {
				t.Fatal(err)
			}
			if tc.field == -1 {
				record = record[:1]
			} else {
				record[tc.field] = "invalid-number"
			}
			broken, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if err := client.Set(ctx, cacheKeys[0], broken, time.Hour).Err(); err != nil {
				t.Fatal(err)
			}
			result, err := podiumredis.IncrementOnceWithTieBreak(ctx, client, keys, "alice", 10, "event", 60, -1)
			if result != nil || err == nil {
				t.Fatalf("corrupt replay record was accepted: %+v, %v", result, err)
			}
			members, err := store.GetMembersWithTieBreak(ctx, keys, "desc", true, "alice")
			if err != nil || len(members) != 1 || members[0] == nil {
				t.Fatalf("read original member: %+v, %v", members, err)
			}
			if members[0].Score != original.Score || !members[0].TTL.Equal(original.TTL) {
				t.Fatalf("corrupt replay changed the committed score or TTL: %+v", members[0])
			}
		})
	}
}
