package database

import (
	"context"
	"fmt"

	podiumredis "github.com/TeneficGames/podium/leaderboard/database/redis"
	goredis "github.com/redis/go-redis/v9"
)

// IncrementMemberScoreOnce persists the increment, TTLs, and retry response together.
func (r *Redis) IncrementMemberScoreOnce(ctx context.Context, leaderboard, member string, increment float64, requestKey string, scoreTTL, expireAt int64) (*Member, error) {
	client, ok := r.Client.(goredis.Scripter)
	if !ok {
		return nil, fmt.Errorf("Redis client does not support idempotent increments")
	}
	result, err := podiumredis.IncrementOnceWithTieBreak(ctx, client, leaderboardKeys(leaderboard), member, increment, requestKey, scoreTTL, expireAt)
	if err != nil {
		return nil, err
	}
	if scoreTTL != 0 {
		// Retry this idempotent registration even on replays after a partial network failure.
		if err := r.Client.SAdd(ctx, ExpirationSet, leaderboard); err != nil {
			return nil, err
		}
	}
	return &Member{Member: result.Member, Score: result.Score, Rank: result.Rank, TTL: result.TTL}, nil
}
