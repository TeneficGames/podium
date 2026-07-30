package redis

import (
	"context"
	"time"
)

const (
	// TTLKeyNotFound is redis return status to TTL command that simbolize a key not found
	TTLKeyNotFound = -2
	// KeyWithoutTTL is redis return status to TTL command that simbolize a key without TTL set
	KeyWithoutTTL = -1
)

// Client interface defines which Redis methods the leaderboard module uses.
type Client interface {
	Del(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) error
	ExpireAt(ctx context.Context, key string, time time.Time) error
	Ping(ctx context.Context) (string, error)
	SAdd(ctx context.Context, key, member string) error
	SMembers(ctx context.Context, key string) ([]string, error)
	SRem(ctx context.Context, key string, members ...string) error
	TTL(ctx context.Context, key string) (time.Duration, error)
	ZAdd(ctx context.Context, key string, members ...*Member) error
	ZCard(ctx context.Context, key string) (int64, error)
	ZIncrBy(ctx context.Context, key, member string, increment float64) error
	ZRange(ctx context.Context, key string, start, stop int64) ([]*Member, error)
	ZRangeByScore(ctx context.Context, key string, min, max string, offset, count int64) ([]string, error)
	ZRank(ctx context.Context, key, member string) (int64, error)
	ZRem(ctx context.Context, key string, members ...string) error
	ZRevRange(ctx context.Context, key string, start, stop int64) ([]*Member, error)
	ZRevRangeByScore(ctx context.Context, key string, min, max string, offset, count int64) ([]string, error)
	ZRevRank(ctx context.Context, key, member string) (int64, error)
	ZScore(ctx context.Context, key, member string) (float64, error)
}

// MemberReader batches score, rank, and optional TTL lookups.
type MemberReader interface {
	ZMembers(ctx context.Context, key, order string, includeTTL bool, members ...string) ([]*Member, error)
}

// MemberWriter batches score updates and their resulting rank lookups.
type MemberWriter interface {
	ZAddAndRanks(ctx context.Context, key, order string, members ...*Member) ([]int64, error)
}

// MemberIncrementer batches a score increment and its resulting rank lookup.
type MemberIncrementer interface {
	ZIncrByAndRank(ctx context.Context, key, member, order string, increment float64) (*Member, error)
}

// TieBreakKeys contains the colocated Redis keys used by a leaderboard.
type TieBreakKeys struct {
	Scores    string
	ScoresAsc string
	Members   string
	Sequence  string
	TTL       string
}

// TieBreakStore provides atomic leaderboard operations whose equal scores are
// ordered by when each member reached its current score.
type TieBreakStore interface {
	DeleteLeaderboard(ctx context.Context, keys TieBreakKeys) error
	ExpireMembersWithTieBreak(ctx context.Context, keys TieBreakKeys, members ...string) error
	ExpireTieBreakKeysAt(ctx context.Context, keys TieBreakKeys, expireAt time.Time) error
	GetMembersWithTieBreak(
		ctx context.Context,
		keys TieBreakKeys,
		order string,
		includeTTL bool,
		members ...string,
	) ([]*Member, error)
	GetRankWithTieBreak(ctx context.Context, keys TieBreakKeys, member, order string) (int64, error)
	IncrementWithTieBreak(
		ctx context.Context,
		keys TieBreakKeys,
		member, order string,
		increment float64,
	) (*Member, error)
	SetMembersTTLWithTieBreak(ctx context.Context, keys TieBreakKeys, members ...*Member) error
	UpsertMembersWithTieBreak(
		ctx context.Context,
		keys TieBreakKeys,
		order string,
		members ...*Member,
	) ([]int64, error)
}

// Member is a struct to be used by sorted set range operations
type Member struct {
	Member string
	Score  float64
	Rank   int64
	TTL    time.Time
}
