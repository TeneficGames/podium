package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type clusterClient struct {
	*goredis.ClusterClient
}

var _ Client = &clusterClient{}
var _ MemberReader = &clusterClient{}
var _ MemberWriter = &clusterClient{}
var _ MemberIncrementer = &clusterClient{}
var _ TieBreakStore = &clusterClient{}

// ClusterOptions define configuration parameters to instantiate a new ClusterClient
type ClusterOptions struct {
	Addrs    []string
	Password string
}

// NewClusterClient returns a new redis cluster client instance
func NewClusterClient(clusterOptions ClusterOptions) Client {
	goRedisClient := goredis.NewClusterClient(&goredis.ClusterOptions{
		Addrs:    clusterOptions.Addrs,
		Password: clusterOptions.Password,
	})

	return &clusterClient{goRedisClient}
}

func (cc *clusterClient) DeleteLeaderboard(ctx context.Context, keys TieBreakKeys) error {
	if err := cc.ClusterClient.Del(
		ctx,
		keys.Scores,
		keys.ScoresAsc,
		keys.Members,
		keys.Sequence,
		keys.TTL,
	).Err(); err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

func (cc *clusterClient) ExpireMembersWithTieBreak(
	ctx context.Context,
	keys TieBreakKeys,
	members ...string,
) error {
	return removeMembersWithTieBreak(ctx, cc.ClusterClient, keys, members...)
}

func (cc *clusterClient) ExpireTieBreakKeysAt(
	ctx context.Context,
	keys TieBreakKeys,
	expireAt time.Time,
) error {
	return expireTieBreakKeysAt(ctx, cc.ClusterClient, keys, expireAt)
}

func (cc *clusterClient) GetMembersWithTieBreak(
	ctx context.Context,
	keys TieBreakKeys,
	order string,
	includeTTL bool,
	members ...string,
) ([]*Member, error) {
	return getMembersWithTieBreak(ctx, cc.ClusterClient, keys, order, includeTTL, members...)
}

func (cc *clusterClient) GetRankWithTieBreak(
	ctx context.Context,
	keys TieBreakKeys,
	member, order string,
) (int64, error) {
	return getRankWithTieBreak(ctx, cc.ClusterClient, keys, member, order)
}

func (cc *clusterClient) IncrementWithTieBreak(
	ctx context.Context,
	keys TieBreakKeys,
	member, order string,
	increment float64,
) (*Member, error) {
	return incrementWithTieBreak(ctx, cc.ClusterClient, keys, member, order, increment)
}

func (cc *clusterClient) SetMembersTTLWithTieBreak(
	ctx context.Context,
	keys TieBreakKeys,
	members ...*Member,
) error {
	return setMembersTTLWithTieBreak(ctx, cc.ClusterClient, keys, members...)
}

func (cc *clusterClient) UpsertMembersWithTieBreak(
	ctx context.Context,
	keys TieBreakKeys,
	order string,
	members ...*Member,
) ([]int64, error) {
	return upsertMembersWithTieBreak(ctx, cc.ClusterClient, keys, order, members...)
}

// ZMembers returns scores, ranks, and optional TTLs in one Redis pipeline.
func (cc *clusterClient) ZMembers(
	ctx context.Context,
	key, order string,
	includeTTL bool,
	members ...string,
) ([]*Member, error) {
	return getMembers(ctx, cc.ClusterClient, key, order, includeTTL, members...)
}

// ZAddAndRanks updates scores and returns their resulting ranks in one pipeline.
func (cc *clusterClient) ZAddAndRanks(
	ctx context.Context,
	key, order string,
	members ...*Member,
) ([]int64, error) {
	return addMembersAndGetRanks(ctx, cc.ClusterClient, key, order, members...)
}

// ZIncrByAndRank increments a score and returns its value and rank in one pipeline.
func (cc *clusterClient) ZIncrByAndRank(
	ctx context.Context,
	key, member, order string,
	increment float64,
) (*Member, error) {
	return incrementMemberAndGetRank(ctx, cc.ClusterClient, key, member, order, increment)
}

// Del call redis DEL function
func (cc *clusterClient) Del(ctx context.Context, key string) error {
	err := cc.ClusterClient.Del(ctx, key).Err()
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

func (cc *clusterClient) Exists(ctx context.Context, key string) error {
	value, err := cc.ClusterClient.Exists(ctx, key).Result()
	if err != nil {
		return NewGeneralError(err.Error())
	}

	if value != 1 {
		return NewKeyNotFoundError(key)
	}

	return nil
}

// ExpireAt call redis EXPIREAT function
func (cc *clusterClient) ExpireAt(ctx context.Context, key string, time time.Time) error {
	result, err := cc.ClusterClient.ExpireAt(ctx, key, time).Result()
	if err != nil {
		return NewGeneralError(err.Error())
	}

	if !result {
		return NewKeyNotFoundError(key)
	}

	return nil
}

// Ping call redis PING function
func (cc *clusterClient) Ping(ctx context.Context) (string, error) {
	result, err := cc.ClusterClient.Ping(ctx).Result()
	if err != nil {
		return "", NewGeneralError(err.Error())
	}
	return result, nil
}

// SAdd call redis SADD function
func (cc *clusterClient) SAdd(ctx context.Context, key, member string) error {
	err := cc.ClusterClient.SAdd(ctx, key, member).Err()
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// SMembers return all members in a set
func (cc *clusterClient) SMembers(ctx context.Context, key string) ([]string, error) {
	result, err := cc.ClusterClient.SMembers(ctx, key).Result()
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}
	return result, nil
}

// SRem call redis SREM function
func (cc *clusterClient) SRem(ctx context.Context, key string, members ...string) error {
	err := cc.ClusterClient.SRem(ctx, key, members).Err()
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// TTL call redis TTL function
func (cc *clusterClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	result, err := cc.ClusterClient.TTL(ctx, key).Result()
	if err != nil {
		return -1, NewGeneralError(err.Error())
	}

	if result == TTLKeyNotFound {
		return -1, NewKeyNotFoundError(key)
	}

	if result == KeyWithoutTTL {
		return -1, NewTTLNotFoundError(key)
	}

	return result, nil
}

// ZAdd call redis ZADD function
func (cc *clusterClient) ZAdd(ctx context.Context, key string, members ...*Member) error {
	goRedisMembers := make([]goredis.Z, 0, len(members))
	for _, member := range members {
		goRedisMembers = append(goRedisMembers, goredis.Z{
			Member: member.Member,
			Score:  member.Score,
		})
	}

	err := cc.ClusterClient.ZAdd(ctx, key, goRedisMembers...).Err()
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// ZCard call redis ZCARD function
func (cc *clusterClient) ZCard(ctx context.Context, key string) (int64, error) {
	result, err := cc.ClusterClient.ZCard(ctx, key).Result()
	if err != nil {
		return -1, NewGeneralError(err.Error())
	}

	if result == 0 {
		return -1, NewKeyNotFoundError(key)
	}

	return result, nil
}

// ZIncrBy call redis ZINCRBY function
func (cc *clusterClient) ZIncrBy(ctx context.Context, key, member string, increment float64) error {
	_, err := cc.ClusterClient.ZIncrBy(ctx, key, increment, member).Result()
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// ZRange call redis ZRANGE function it is inclusive it returns start and stop element
func (cc *clusterClient) ZRange(ctx context.Context, key string, start, stop int64) ([]*Member, error) {
	result, err := cc.ClusterClient.ZRangeWithScores(ctx, key, start, stop).Result()
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}

	members := make([]*Member, 0, len(result))
	for _, member := range result {
		members = append(members, &Member{
			Member: fmt.Sprint(member.Member),
			Score:  member.Score,
		})
	}

	return members, nil
}

// ZRangeByScore call redis ZREVRANGEBYSCORE command
func (cc *clusterClient) ZRangeByScore(ctx context.Context, key string, min, max string, offset, count int64) ([]string, error) {
	result, err := cc.ClusterClient.ZRangeArgs(ctx, goredis.ZRangeArgs{
		Key: key, Start: min, Stop: max, ByScore: true, Offset: offset, Count: count,
	}).Result()
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}
	return result, nil
}

// ZRank call redis ZRANK function
func (cc *clusterClient) ZRank(ctx context.Context, key, member string) (int64, error) {
	result, err := cc.ClusterClient.ZRank(ctx, key, member).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return -1, NewMemberNotFoundError(key, member)
		}

		return -1, NewGeneralError(err.Error())
	}

	return result, nil
}

// ZRem call redis ZREM function
func (cc *clusterClient) ZRem(ctx context.Context, key string, members ...string) error {
	err := cc.ClusterClient.ZRem(ctx, key, members).Err()
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// ZRevRange call redis ZREVRANGE function it is inclusive it returns start and stop element
func (cc *clusterClient) ZRevRange(ctx context.Context, key string, start, stop int64) ([]*Member, error) {
	result, err := cc.ClusterClient.ZRevRangeWithScores(ctx, key, start, stop).Result()
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}

	members := make([]*Member, 0, len(result))
	for _, member := range result {
		members = append(members, &Member{
			Member: fmt.Sprint(member.Member),
			Score:  member.Score,
		})
	}

	return members, nil
}

// ZRevRangeByScore call redis ZREVRANGEBYSCORE command
func (cc *clusterClient) ZRevRangeByScore(ctx context.Context, key string, min, max string, offset, count int64) ([]string, error) {
	result, err := cc.ClusterClient.ZRangeArgs(ctx, goredis.ZRangeArgs{
		Key: key, Start: max, Stop: min, ByScore: true, Rev: true, Offset: offset, Count: count,
	}).Result()
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}
	return result, nil
}

// ZRevRank call redis ZRevRank function
func (cc *clusterClient) ZRevRank(ctx context.Context, key, member string) (int64, error) {
	result, err := cc.ClusterClient.ZRevRank(ctx, key, member).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return -1, NewMemberNotFoundError(key, member)
		}

		return -1, NewGeneralError(err.Error())
	}

	return result, nil
}

// ZScore call redis ZScore function
func (cc *clusterClient) ZScore(ctx context.Context, key, member string) (float64, error) {
	result, err := cc.ClusterClient.ZScore(ctx, key, member).Result()
	if err != nil {
		if err.Error() == "redis: nil" {
			return -1, NewMemberNotFoundError(key, member)
		}

		return -1, NewGeneralError(err.Error())
	}

	return result, nil
}
