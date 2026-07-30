package redis_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	podiumredis "github.com/TeneficGames/podium/leaderboard/database/redis"
	podiumtesting "github.com/TeneficGames/podium/leaderboard/testing"
	goredis "github.com/redis/go-redis/v9"
)

const (
	packedSequenceCapacity = int64(1_000_000_000)
	packedMaxAbsoluteScore = int64(9_007_198)
	benchmarkSeedSize      = 1_000
)

var benchmarkKeySequence atomic.Uint64

var encodedUpdateScript = goredis.NewScript(`
local old = redis.call("HGET", KEYS[3], ARGV[1])
local function token(sequence)
  return string.format("%016d", 9007199254740991 - sequence) .. ARGV[1]
end

local oldToken = nil
if old then
  local separator = string.find(old, ":")
  local oldSequence = tonumber(string.sub(old, 1, separator - 1))
  local oldScore = string.sub(old, separator + 1)
  oldToken = token(oldSequence)
  if oldScore == ARGV[2] then
    return redis.call("ZREVRANK", KEYS[1], oldToken)
  end
end

local sequence = redis.call("INCR", KEYS[2])
if sequence > 9007199254740991 then
  return redis.error_reply("encoded sequence capacity exceeded")
end
if oldToken then
  redis.call("ZREM", KEYS[1], oldToken)
end
local newToken = token(sequence)
redis.call("ZADD", KEYS[1], ARGV[2], newToken)
redis.call("HSET", KEYS[3], ARGV[1], sequence .. ":" .. ARGV[2])
return redis.call("ZREVRANK", KEYS[1], newToken)
`)

var encodedRankScript = goredis.NewScript(`
local metadata = redis.call("HGET", KEYS[2], ARGV[1])
if not metadata then
  return nil
end
local separator = string.find(metadata, ":")
local sequence = tonumber(string.sub(metadata, 1, separator - 1))
local token = string.format("%016d", 9007199254740991 - sequence) .. ARGV[1]
return redis.call("ZREVRANK", KEYS[1], token)
`)

var packedUpdateScript = goredis.NewScript(`
local old = redis.call("ZSCORE", KEYS[1], ARGV[1])
local capacity = tonumber(ARGV[3])
local score = tonumber(ARGV[2])
if math.abs(score) > tonumber(ARGV[4]) then
  return redis.error_reply("score exceeds packed representation")
end
if old and math.floor(tonumber(old) / capacity) == score then
  return redis.call("ZREVRANK", KEYS[1], ARGV[1])
end

local sequence = redis.call("INCR", KEYS[2])
if sequence >= capacity then
  return redis.error_reply("packed sequence capacity exceeded")
end
local composite = score * capacity + (capacity - sequence)
redis.call("ZADD", KEYS[1], composite, ARGV[1])
return redis.call("ZREVRANK", KEYS[1], ARGV[1])
`)

type tieBreakStrategy interface {
	update(context.Context, string, int64) (int64, error)
	rank(context.Context, string) (int64, error)
	top(context.Context, int64) ([]string, error)
	memoryUsage(context.Context) (int64, error)
	close(context.Context)
}

type encodedMemberStrategy struct {
	client      *goredis.Client
	scoresKey   string
	sequenceKey string
	membersKey  string
}

type productionEncodedMemberStrategy struct {
	rawClient *goredis.Client
	store     podiumredis.TieBreakStore
	keys      podiumredis.TieBreakKeys
}

func (s *productionEncodedMemberStrategy) update(ctx context.Context, member string, score int64) (int64, error) {
	ranks, err := s.store.UpsertMembersWithTieBreak(ctx, s.keys, "desc",
		&podiumredis.Member{Member: member, Score: float64(score)},
	)
	if err != nil {
		return 0, err
	}
	return ranks[0], nil
}

func (s *productionEncodedMemberStrategy) rank(ctx context.Context, member string) (int64, error) {
	return s.store.GetRankWithTieBreak(ctx, s.keys, member, "desc")
}

func (s *productionEncodedMemberStrategy) top(ctx context.Context, count int64) ([]string, error) {
	members, err := s.rawClient.ZRangeArgs(ctx, goredis.ZRangeArgs{
		Key:   s.keys.Scores,
		Start: 0,
		Stop:  count - 1,
		Rev:   true,
	}).Result()
	if err != nil {
		return nil, err
	}
	for i := range members {
		members[i] = members[i][19:]
	}
	return members, nil
}

func (s *productionEncodedMemberStrategy) memoryUsage(ctx context.Context) (int64, error) {
	return sumMemoryUsage(
		ctx,
		s.rawClient,
		s.keys.Scores,
		s.keys.ScoresAsc,
		s.keys.Sequence,
		s.keys.Members,
	)
}

func (s *productionEncodedMemberStrategy) close(ctx context.Context) {
	_ = s.store.DeleteLeaderboard(ctx, s.keys)
}

func (s *encodedMemberStrategy) update(ctx context.Context, member string, score int64) (int64, error) {
	return encodedUpdateScript.Run(
		ctx,
		s.client,
		[]string{s.scoresKey, s.sequenceKey, s.membersKey},
		member,
		strconv.FormatInt(score, 10),
	).Int64()
}

func (s *encodedMemberStrategy) rank(ctx context.Context, member string) (int64, error) {
	return encodedRankScript.Run(
		ctx,
		s.client,
		[]string{s.scoresKey, s.membersKey},
		member,
	).Int64()
}

func (s *encodedMemberStrategy) top(ctx context.Context, count int64) ([]string, error) {
	members, err := s.client.ZRangeArgs(ctx, goredis.ZRangeArgs{
		Key:   s.scoresKey,
		Start: 0,
		Stop:  count - 1,
		Rev:   true,
	}).Result()
	if err != nil {
		return nil, err
	}
	for i := range members {
		members[i] = members[i][16:]
	}
	return members, nil
}

func (s *encodedMemberStrategy) memoryUsage(ctx context.Context) (int64, error) {
	return sumMemoryUsage(ctx, s.client, s.scoresKey, s.sequenceKey, s.membersKey)
}

func (s *encodedMemberStrategy) close(ctx context.Context) {
	_ = s.client.Del(ctx, s.scoresKey, s.sequenceKey, s.membersKey).Err()
}

type packedScoreStrategy struct {
	client      *goredis.Client
	scoresKey   string
	sequenceKey string
}

func (s *packedScoreStrategy) update(ctx context.Context, member string, score int64) (int64, error) {
	return packedUpdateScript.Run(
		ctx,
		s.client,
		[]string{s.scoresKey, s.sequenceKey},
		member,
		strconv.FormatInt(score, 10),
		packedSequenceCapacity,
		packedMaxAbsoluteScore,
	).Int64()
}

func (s *packedScoreStrategy) rank(ctx context.Context, member string) (int64, error) {
	return s.client.ZRevRank(ctx, s.scoresKey, member).Result()
}

func (s *packedScoreStrategy) top(ctx context.Context, count int64) ([]string, error) {
	return s.client.ZRangeArgs(ctx, goredis.ZRangeArgs{
		Key:   s.scoresKey,
		Start: 0,
		Stop:  count - 1,
		Rev:   true,
	}).Result()
}

func (s *packedScoreStrategy) memoryUsage(ctx context.Context) (int64, error) {
	return sumMemoryUsage(ctx, s.client, s.scoresKey, s.sequenceKey)
}

func (s *packedScoreStrategy) close(ctx context.Context) {
	_ = s.client.Del(ctx, s.scoresKey, s.sequenceKey).Err()
}

type plainScoreStrategy struct {
	client    *goredis.Client
	scoresKey string
}

func (s *plainScoreStrategy) update(ctx context.Context, member string, score int64) (int64, error) {
	var rank *goredis.IntCmd
	_, err := s.client.Pipelined(ctx, func(pipe goredis.Pipeliner) error {
		pipe.ZAdd(ctx, s.scoresKey, goredis.Z{Member: member, Score: float64(score)})
		rank = pipe.ZRevRank(ctx, s.scoresKey, member)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return rank.Result()
}

func (s *plainScoreStrategy) rank(ctx context.Context, member string) (int64, error) {
	return s.client.ZRevRank(ctx, s.scoresKey, member).Result()
}

func (s *plainScoreStrategy) top(ctx context.Context, count int64) ([]string, error) {
	return s.client.ZRangeArgs(ctx, goredis.ZRangeArgs{
		Key:   s.scoresKey,
		Start: 0,
		Stop:  count - 1,
		Rev:   true,
	}).Result()
}

func (s *plainScoreStrategy) memoryUsage(ctx context.Context) (int64, error) {
	return sumMemoryUsage(ctx, s.client, s.scoresKey)
}

func (s *plainScoreStrategy) close(ctx context.Context) {
	_ = s.client.Del(ctx, s.scoresKey).Err()
}

func TestTieBreakStrategies(t *testing.T) {
	server, err := podiumtesting.StartRedis()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()

	for _, strategyName := range []string{"production-encoded-member", "encoded-member", "packed-score"} {
		t.Run(strategyName, func(t *testing.T) {
			strategy := newTieBreakStrategy(client, strategyName)
			t.Cleanup(func() { strategy.close(ctx) })

			mustUpdate(t, strategy, "alice", 100)
			mustUpdate(t, strategy, "bob", 100)
			assertTop(t, strategy, "alice", "bob")

			mustUpdate(t, strategy, "alice", 100)
			assertTop(t, strategy, "alice", "bob")

			mustUpdate(t, strategy, "alice", 90)
			mustUpdate(t, strategy, "alice", 100)
			assertTop(t, strategy, "bob", "alice")

			mustUpdate(t, strategy, "charlie", 101)
			assertTop(t, strategy, "charlie", "bob", "alice")

			if strategyName == "packed-score" {
				if _, err := strategy.update(ctx, "too-large", packedMaxAbsoluteScore+1); err == nil {
					t.Fatal("expected packed representation to reject an out-of-range score")
				}

				packed := strategy.(*packedScoreStrategy)
				if err := client.Set(ctx, packed.sequenceKey, packedSequenceCapacity-1, 0).Err(); err != nil {
					t.Fatal(err)
				}
				if _, err := strategy.update(ctx, "sequence-overflow", 100); err == nil {
					t.Fatal("expected packed representation to reject sequence exhaustion")
				}
			}
		})
	}
}

func BenchmarkTieBreakStrategies(b *testing.B) {
	client := tieBreakRedis(b)

	for _, strategyName := range []string{"plain-score-baseline", "production-encoded-member", "encoded-member", "packed-score"} {
		b.Run(strategyName, func(b *testing.B) {
			b.Run("insert-and-rank", func(b *testing.B) {
				benchmarkInsertAndRank(b, client, strategyName)
			})
			b.Run("change-score-and-rank", func(b *testing.B) {
				benchmarkChangeScoreAndRank(b, client, strategyName)
			})
			b.Run("duplicate-score-and-rank", func(b *testing.B) {
				benchmarkDuplicateScoreAndRank(b, client, strategyName)
			})
			b.Run("get-rank", func(b *testing.B) {
				benchmarkGetRank(b, client, strategyName)
			})
			b.Run("get-top-50", func(b *testing.B) {
				benchmarkGetTop(b, client, strategyName)
			})
		})
	}
}

func benchmarkInsertAndRank(b *testing.B, client *goredis.Client, strategyName string) {
	ctx := context.Background()
	strategy := newTieBreakStrategy(client, strategyName)
	b.Cleanup(func() { strategy.close(ctx) })
	warmStrategy(b, strategy)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := strategy.update(ctx, fmt.Sprintf("member-%d", i), int64(i%1_000)); err != nil {
			b.Fatal(err)
		}
	}

	b.StopTimer()
	reportMemoryPerMember(b, ctx, strategy, b.N)
}

func benchmarkChangeScoreAndRank(b *testing.B, client *goredis.Client, strategyName string) {
	ctx := context.Background()
	strategy := newTieBreakStrategy(client, strategyName)
	b.Cleanup(func() { strategy.close(ctx) })
	seedStrategy(b, strategy, benchmarkSeedSize, 100)
	warmStrategy(b, strategy)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		score := int64(200 + (i/benchmarkSeedSize)%2)
		if _, err := strategy.update(ctx, fmt.Sprintf("member-%d", i%benchmarkSeedSize), score); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkDuplicateScoreAndRank(b *testing.B, client *goredis.Client, strategyName string) {
	ctx := context.Background()
	strategy := newTieBreakStrategy(client, strategyName)
	b.Cleanup(func() { strategy.close(ctx) })
	seedStrategy(b, strategy, benchmarkSeedSize, 100)
	warmStrategy(b, strategy)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := strategy.update(ctx, fmt.Sprintf("member-%d", i%benchmarkSeedSize), 100); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkGetRank(b *testing.B, client *goredis.Client, strategyName string) {
	ctx := context.Background()
	strategy := newTieBreakStrategy(client, strategyName)
	b.Cleanup(func() { strategy.close(ctx) })
	seedStrategy(b, strategy, benchmarkSeedSize, 100)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := strategy.rank(ctx, fmt.Sprintf("member-%d", i%benchmarkSeedSize)); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkGetTop(b *testing.B, client *goredis.Client, strategyName string) {
	ctx := context.Background()
	strategy := newTieBreakStrategy(client, strategyName)
	b.Cleanup(func() { strategy.close(ctx) })
	seedStrategy(b, strategy, benchmarkSeedSize, 100)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := strategy.top(ctx, 50); err != nil {
			b.Fatal(err)
		}
	}
}

func tieBreakRedis(tb testing.TB) *goredis.Client {
	tb.Helper()
	address := os.Getenv("PODIUM_BENCH_REDIS_ADDR")
	if address == "" {
		address = "localhost:6379"
	}
	client := goredis.NewClient(&goredis.Options{Addr: address})
	tb.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		tb.Skipf("Redis is unavailable at %s: %v", address, err)
	}
	return client
}

func newTieBreakStrategy(client *goredis.Client, strategyName string) tieBreakStrategy {
	id := benchmarkKeySequence.Add(1)
	tag := fmt.Sprintf("tiebreak-%d-%d", os.Getpid(), id)
	prefix := "podium:{" + tag + "}"
	switch strategyName {
	case "plain-score-baseline":
		return &plainScoreStrategy{
			client:    client,
			scoresKey: prefix + ":scores",
		}
	case "encoded-member":
		return &encodedMemberStrategy{
			client:      client,
			scoresKey:   prefix + ":scores",
			sequenceKey: prefix + ":sequence",
			membersKey:  prefix + ":members",
		}
	case "production-encoded-member":
		options := client.Options()
		host, portString, found := strings.Cut(options.Addr, ":")
		if !found {
			panic("Redis benchmark address must be host:port")
		}
		port, err := strconv.Atoi(portString)
		if err != nil {
			panic(err)
		}
		podiumClient := podiumredis.NewStandaloneClient(podiumredis.StandaloneOptions{
			Host: host, Port: port, Password: options.Password, DB: options.DB,
		})
		store, ok := podiumClient.(podiumredis.TieBreakStore)
		if !ok {
			panic("standalone Redis client does not implement TieBreakStore")
		}
		return &productionEncodedMemberStrategy{
			rawClient: client,
			store:     store,
			keys: podiumredis.TieBreakKeys{
				Scores:    prefix + ":scores",
				ScoresAsc: prefix + ":scores-asc",
				Sequence:  prefix + ":sequence",
				Members:   prefix + ":members",
				TTL:       prefix + ":ttl",
			},
		}
	case "packed-score":
		return &packedScoreStrategy{
			client:      client,
			scoresKey:   prefix + ":scores",
			sequenceKey: prefix + ":sequence",
		}
	default:
		panic("unknown tie-break strategy: " + strategyName)
	}
}

func seedStrategy(tb testing.TB, strategy tieBreakStrategy, count int, score int64) {
	tb.Helper()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		if _, err := strategy.update(ctx, fmt.Sprintf("member-%d", i), score); err != nil {
			tb.Fatal(err)
		}
	}
}

func warmStrategy(tb testing.TB, strategy tieBreakStrategy) {
	tb.Helper()
	if _, err := strategy.update(context.Background(), "warm-member", 1); err != nil {
		tb.Fatal(err)
	}
}

func mustUpdate(t *testing.T, strategy tieBreakStrategy, member string, score int64) {
	t.Helper()
	if _, err := strategy.update(context.Background(), member, score); err != nil {
		t.Fatal(err)
	}
}

func assertTop(t *testing.T, strategy tieBreakStrategy, expected ...string) {
	t.Helper()
	actual, err := strategy.top(context.Background(), int64(len(expected)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(actual, ",") != strings.Join(expected, ",") {
		t.Fatalf("expected top members %v, got %v", expected, actual)
	}
}

func sumMemoryUsage(ctx context.Context, client *goredis.Client, keys ...string) (int64, error) {
	var total int64
	for _, key := range keys {
		usage, err := client.MemoryUsage(ctx, key, 0).Result()
		if err != nil && !errors.Is(err, goredis.Nil) {
			return 0, err
		}
		total += usage
	}
	return total, nil
}

func reportMemoryPerMember(b *testing.B, ctx context.Context, strategy tieBreakStrategy, members int) {
	if members == 0 {
		return
	}
	memory, err := strategy.memoryUsage(ctx)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportMetric(float64(memory)/float64(members), "redis-B/member")
}
