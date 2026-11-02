package database

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	podiumredis "github.com/TeneficGames/podium/leaderboard/database/redis"
	"github.com/TeneficGames/podium/leaderboard/expiration"
	"github.com/TeneficGames/podium/leaderboard/model"
	podiumtesting "github.com/TeneficGames/podium/leaderboard/testing"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/mock/gomock"
)

func newIdempotencyDatabase(t *testing.T) (*Redis, *podiumtesting.RedisServer, *goredis.Client) {
	t.Helper()
	server, err := podiumtesting.StartRedis()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	db := NewRedisDatabase(RedisOptions{Host: server.Host, Port: server.Port})
	raw := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	client := db.Client.(interface{ Close() error })
	t.Cleanup(func() { _ = raw.Close(); _ = client.Close() })
	return db, server, raw
}

func TestIdempotentIncrementConcurrencyAndReplay(t *testing.T) {
	db, server, _ := newIdempotencyDatabase(t)
	second := NewRedisDatabase(RedisOptions{Host: server.Host, Port: server.Port})
	t.Cleanup(func() { _ = second.Client.(interface{ Close() error }).Close() })
	ctx := context.Background()
	const count = 32
	results := make(chan *Member, count)
	errorsCh := make(chan error, count)
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			writer := db
			if i%2 != 0 {
				writer = second
			}
			result, err := writer.IncrementMemberScoreOnce(ctx, "board", "alice", 100, "event", 60, -1)
			results <- result
			errorsCh <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	var original *Member
	for result := range results {
		if original == nil {
			original = result
		}
		if !reflect.DeepEqual(original, result) {
			t.Fatalf("different retry result: %+v / %+v", original, result)
		}
	}
	if original.Score != 100 || original.Rank != 0 || original.TTL.IsZero() {
		t.Fatalf("unexpected original result: %+v", original)
	}
	if err := db.SetMembers(ctx, "board", []*Member{{Member: "bob", Score: 200}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.IncrementMemberScoreOnce(ctx, "board", "alice", 5, "other-event", 0, -1); err != nil {
		t.Fatal(err)
	}
	server.FastForward(10 * time.Second)
	replayed, err := second.IncrementMemberScoreOnce(ctx, "board", "alice", 100, "event", 60, -1)
	if err != nil || !reflect.DeepEqual(original, replayed) {
		t.Fatalf("lost original response: %+v, %v", replayed, err)
	}
	members, err := db.GetMembers(ctx, "board", "desc", true, "alice")
	if err != nil || members[0].Score != 105 || !members[0].TTL.Equal(original.TTL) {
		t.Fatalf("retry changed score or TTL: %+v, %v", members, err)
	}
	for _, change := range []struct {
		member    string
		increment float64
		ttl       int64
	}{{"bob", 100, 60}, {"alice", 101, 60}, {"alice", 100, 61}} {
		_, err := db.IncrementMemberScoreOnce(ctx, "board", change.member, change.increment, "event", change.ttl, -1)
		if !errors.Is(err, model.ErrIdempotencyConflict) {
			t.Fatalf("expected payload conflict, got %v", err)
		}
	}
	other, err := db.IncrementMemberScoreOnce(ctx, "another-board", "alice", 100, "event", 0, -1)
	if err != nil || other.Score != 100 {
		t.Fatalf("key collided across leaderboards: %+v, %v", other, err)
	}
	server.FastForward(24 * time.Hour)
	fresh, err := db.IncrementMemberScoreOnce(ctx, "board", "alice", 100, "event", 60, -1)
	if err != nil || fresh.Score != 205 {
		t.Fatalf("expired key could not represent a new event: %+v, %v", fresh, err)
	}
}

type registrationFailureClient struct {
	podiumredis.Client
	podiumredis.TieBreakStore
	goredis.Scripter
	fail bool
}

func (c *registrationFailureClient) SAdd(ctx context.Context, key, member string) error {
	if c.fail {
		c.fail = false
		return fmt.Errorf("expiration registration unavailable")
	}
	return c.Client.SAdd(ctx, key, member)
}

func TestIdempotentIncrementRepairsTTLRegistration(t *testing.T) {
	db, server, _ := newIdempotencyDatabase(t)
	db.Client = &registrationFailureClient{Client: db.Client, TieBreakStore: db.Client.(podiumredis.TieBreakStore), Scripter: db.Client.(goredis.Scripter), fail: true}
	ctx := context.Background()
	if _, err := db.IncrementMemberScoreOnce(ctx, "repair", "alice", 100, "event", 60, -1); err == nil {
		t.Fatal("expected registration failure")
	}
	members, err := db.GetMembers(ctx, "repair", "desc", true, "alice")
	if err != nil || members[0].Score != 100 {
		t.Fatalf("expected committed score: %+v, %v", members, err)
	}
	expireAt := members[0].TTL
	server.FastForward(10 * time.Second)
	result, err := db.IncrementMemberScoreOnce(ctx, "repair", "alice", 100, "event", 60, -1)
	if err != nil || result.Score != 100 || !result.TTL.Equal(expireAt) {
		t.Fatalf("retry changed committed result: %+v, %v", result, err)
	}
	sets, err := db.SMembers(ctx, ExpirationSet)
	if err != nil || len(sets) != 1 || sets[0] != "repair" {
		t.Fatalf("registration not repaired: %v, %v", sets, err)
	}
}

func TestIdempotentIncrementFailuresAndSeasonTTL(t *testing.T) {
	db, _, raw := newIdempotencyDatabase(t)
	ctx := context.Background()
	keys := leaderboardKeys("failures")
	if err := db.SetMembers(ctx, "failures", []*Member{{Member: "alice", Score: model.MaxScore}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.IncrementMemberScoreOnce(ctx, "failures", "alice", 1, "event", 60, -1); !errors.Is(err, model.ErrInvalidScore) {
		t.Fatalf("expected range error: %v", err)
	}
	if err := raw.Set(ctx, keys.TTL, "wrong type", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.IncrementMemberScoreOnce(ctx, "failures", "alice", -1, "event", 60, -1); err == nil {
		t.Fatal("expected TTL key error")
	}
	members, err := db.GetMembers(ctx, "failures", "desc", false, "alice")
	if err != nil || members[0].Score != model.MaxScore {
		t.Fatalf("failed script changed score: %+v, %v", members, err)
	}
	if err := raw.Del(ctx, keys.TTL).Err(); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour).Unix()
	result, err := db.IncrementMemberScoreOnce(ctx, "failures", "alice", -1, "event", 60, expires)
	if err != nil || result.Score != model.MaxScore-1 {
		t.Fatalf("failed request consumed key: %+v, %v", result, err)
	}
	for _, key := range []string{keys.Scores, keys.ScoresAsc, keys.Members, keys.Sequence, keys.TTL} {
		ttl, err := raw.TTL(ctx, key).Result()
		if err != nil || ttl <= 0 || ttl > time.Hour {
			t.Fatalf("missing season TTL for %s: %v, %v", key, ttl, err)
		}
	}
	if err := raw.Del(ctx, keys.Scores, keys.ScoresAsc, keys.Members, keys.Sequence, keys.TTL).Err(); err != nil {
		t.Fatal(err)
	}
	replay, err := db.IncrementMemberScoreOnce(ctx, "failures", "alice", -1, "event", 60, -2)
	if err != nil || !reflect.DeepEqual(result, replay) {
		t.Fatalf("season expiry blocked replay: %+v, %v", replay, err)
	}
	_, err = db.IncrementMemberScoreOnce(ctx, "failures", "alice", -1, "new-event", 60, -2)
	var expired *expiration.LeaderboardExpiredError
	if !errors.As(err, &expired) {
		t.Fatalf("expired season accepted new increment: %v", err)
	}
	if raw.Exists(ctx, keys.Scores).Val() != 0 {
		t.Fatal("expired season was recreated")
	}
}

func TestIdempotentIncrementRequiresScripting(t *testing.T) {
	client := podiumredis.NewMockRedis(gomock.NewController(t))
	db := &Redis{Client: client}
	result, err := db.IncrementMemberScoreOnce(context.Background(), "board", "alice", 10, "event", 0, -1)
	if result != nil || err == nil {
		t.Fatalf("non-scripting clients must reject instead of using a non-atomic fallback: %+v, %v", result, err)
	}
}
