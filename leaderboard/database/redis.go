package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TeneficGames/podium/leaderboard/database/redis"
)

// Redis is a type that implements Database interface with redis client
type Redis struct {
	redis.Client
}

// ExpirationSet is used to list expirations set that worker will use to remove members
const ExpirationSet string = "expiration-sets"

// RedisOptions is a struct to create a new redis client
type RedisOptions struct {
	ClusterEnabled bool
	Addrs          []string
	Host           string
	Port           int
	Password       string
	DB             int
}

// NewRedisDatabase create a database based on redis
func NewRedisDatabase(options RedisOptions) *Redis {
	if options.ClusterEnabled {
		return &Redis{redis.NewClusterClient(redis.ClusterOptions{
			Addrs:    options.Addrs,
			Password: options.Password,
		})}
	}

	return &Redis{redis.NewStandaloneClient(redis.StandaloneOptions{
		Host:     options.Host,
		Port:     options.Port,
		Password: options.Password,
		DB:       options.DB,
	})}
}

// GetLeaderboardExpiration return leaderboard expiration time
func (r *Redis) GetLeaderboardExpiration(ctx context.Context, leaderboard string) (int64, error) {
	duration, err := r.Client.TTL(ctx, leaderboard)
	if err != nil {
		var notFoundErr *redis.TTLNotFoundError
		if errors.As(err, &notFoundErr) {
			return int64(-1), NewTTLNotFoundError(leaderboard)
		}
		return int64(-1), NewGeneralError(err.Error())
	}

	return int64(duration), nil
}

// GetMembers return members from leaderboard
func (r *Redis) GetMembers(ctx context.Context, leaderboard, order string, includeTTL bool, members ...string) ([]*Member, error) {
	if order != "asc" && order != "desc" {
		return nil, NewInvalidOrderError(order)
	}

	if reader, ok := r.Client.(redis.MemberReader); ok {
		redisMembers, err := reader.ZMembers(ctx, leaderboard, order, includeTTL, members...)
		if err != nil {
			return nil, NewGeneralError(err.Error())
		}

		membersToReturn := make([]*Member, len(redisMembers))
		for i, member := range redisMembers {
			if member == nil {
				continue
			}
			membersToReturn[i] = &Member{
				Member: member.Member,
				Score:  member.Score,
				Rank:   member.Rank,
				TTL:    member.TTL,
			}
		}
		return membersToReturn, nil
	}

	membersToReturn := make([]*Member, 0, len(members))

	for _, member := range members {
		score, err := r.Client.ZScore(ctx, leaderboard, member)
		if err != nil {
			var notFoundErr *redis.MemberNotFoundError
			if errors.As(err, &notFoundErr) {
				membersToReturn = append(membersToReturn, nil)
				continue
			}

			return nil, NewGeneralError(err.Error())
		}

		var rank int64
		switch order {
		case "asc":
			rank, err = r.Client.ZRank(ctx, leaderboard, member)
		case "desc":
			rank, err = r.Client.ZRevRank(ctx, leaderboard, member)
		}
		if err != nil {
			return nil, NewGeneralError(err.Error())
		}

		var ttl time.Time
		if includeTTL {
			ttl, err = r.getMemberTTL(ctx, leaderboard, member)
			if err != nil {
				var notFoundErr *MemberNotFoundError
				if !errors.As(err, &notFoundErr) {
					return nil, NewGeneralError(err.Error())
				}

				ttl = time.Time{}
			}
		}

		membersToReturn = append(membersToReturn, &Member{
			Member: member,
			Score:  score,
			Rank:   rank,
			TTL:    ttl,
		})

	}
	return membersToReturn, nil
}

func (r *Redis) getMemberTTL(ctx context.Context, leaderboard, member string) (time.Time, error) {
	leaderboardTTL := fmt.Sprintf("%s:ttl", leaderboard)
	ttl, err := r.Client.ZScore(ctx, leaderboardTTL, member)
	if err != nil {
		var notFoundErr *redis.MemberNotFoundError
		if errors.As(err, &notFoundErr) {
			return time.Time{}, NewMemberNotFoundError(leaderboardTTL, member)
		}
		return time.Time{}, NewGeneralError(err.Error())
	}

	return time.Unix(int64(ttl), 0), nil
}

// GetMemberIDsWithScoreInsideRange find members with score close to
func (r *Redis) GetMemberIDsWithScoreInsideRange(ctx context.Context, leaderboard string, min, max string, offset, count int) ([]string, error) {
	members, err := r.Client.ZRevRangeByScore(ctx, leaderboard, min, max, int64(offset), int64(count))
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}

	return members, nil
}

// GetOrderedMembers call redis ZRange if order is asc, if desc call redis ZRevRange
func (r *Redis) GetOrderedMembers(ctx context.Context, leaderboard string, start, stop int, order string) ([]*Member, error) {
	var redisMembers []*redis.Member
	var err error

	switch order {
	case "asc":
		redisMembers, err = r.Client.ZRange(ctx, leaderboard, int64(start), int64(stop))
	case "desc":
		redisMembers, err = r.Client.ZRevRange(ctx, leaderboard, int64(start), int64(stop))
	default:
		return nil, NewInvalidOrderError(order)
	}

	if err != nil {
		return nil, NewGeneralError(err.Error())
	}

	members := make([]*Member, 0, len(redisMembers))
	for i, member := range redisMembers {
		members = append(members, &Member{
			Member: member.Member,
			Score:  member.Score,
			Rank:   int64(start + i),
		})
	}

	return members, nil
}

// GetRank find member position on leaderboard
func (r *Redis) GetRank(ctx context.Context, leaderboard, member, order string) (int, error) {
	var err error
	var rank int64

	switch order {
	case "asc":
		rank, err = r.Client.ZRank(ctx, leaderboard, member)
	case "desc":
		rank, err = r.Client.ZRevRank(ctx, leaderboard, member)
	default:
		return -1, NewInvalidOrderError(order)
	}

	if err != nil {
		var notFoundErr *redis.MemberNotFoundError
		if errors.As(err, &notFoundErr) {
			return -1, NewMemberNotFoundError(leaderboard, member)
		}

		return -1, NewGeneralError(err.Error())
	}

	return int(rank), nil
}

// GetTotalMembers return total members in a leaderboard
func (r *Redis) GetTotalMembers(ctx context.Context, leaderboard string) (int, error) {
	totalMembers, err := r.Client.ZCard(ctx, leaderboard)
	if err != nil {
		var notFoundErr *redis.KeyNotFoundError
		if errors.As(err, &notFoundErr) {
			return 0, nil
		}
		return -1, NewGeneralError(err.Error())
	}

	return int(totalMembers), nil
}

// Healthcheck is a function that call redis ping to understand if redis is ok
func (r *Redis) Healthcheck(ctx context.Context) error {
	_, err := r.Ping(ctx)
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// IncrementMemberScore add to member score the value in parameter
func (r *Redis) IncrementMemberScore(ctx context.Context, leaderboard, member string, increment float64) error {
	err := r.ZIncrBy(ctx, leaderboard, member, increment)
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// IncrementMemberScoreAndGetRank increments a score and returns its resulting value and rank.
func (r *Redis) IncrementMemberScoreAndGetRank(
	ctx context.Context,
	leaderboard, member, order string,
	increment float64,
) (*Member, error) {
	if incrementer, ok := r.Client.(redis.MemberIncrementer); ok {
		redisMember, err := incrementer.ZIncrByAndRank(ctx, leaderboard, member, order, increment)
		if err != nil {
			return nil, NewGeneralError(err.Error())
		}
		return &Member{
			Member: redisMember.Member,
			Score:  redisMember.Score,
			Rank:   redisMember.Rank,
		}, nil
	}

	if err := r.IncrementMemberScore(ctx, leaderboard, member, increment); err != nil {
		return nil, err
	}
	members, err := r.GetMembers(ctx, leaderboard, order, false, member)
	if err != nil {
		return nil, err
	}
	return members[0], nil
}

// RemoveLeaderboard delete leaderboard key from redis
func (r *Redis) RemoveLeaderboard(ctx context.Context, leaderboard string) error {
	err := r.Client.Del(ctx, leaderboard)
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// RemoveMembers delete from redis members
func (r *Redis) RemoveMembers(ctx context.Context, leaderboard string, members ...string) error {
	err := r.Client.ZRem(ctx, leaderboard, members...)
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// SetLeaderboardExpiration will set leaderboard expiration time
func (r *Redis) SetLeaderboardExpiration(ctx context.Context, leaderboard string, expireAt time.Time) error {
	err := r.Client.ExpireAt(ctx, leaderboard, expireAt)
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// SetMembers will set member score and ttl
func (r *Redis) SetMembers(ctx context.Context, leaderboard string, databaseMembers []*Member) error {
	redisMembers := make([]*redis.Member, 0, len(databaseMembers))
	for _, member := range databaseMembers {
		redisMembers = append(redisMembers, &redis.Member{
			Member: member.Member,
			Score:  member.Score,
		})
	}
	err := r.Client.ZAdd(ctx, leaderboard, redisMembers...)
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// SetMembersAndGetRanks updates scores and returns their resulting ranks.
func (r *Redis) SetMembersAndGetRanks(
	ctx context.Context,
	leaderboard, order string,
	databaseMembers []*Member,
) ([]int64, error) {
	if writer, ok := r.Client.(redis.MemberWriter); ok {
		redisMembers := make([]*redis.Member, len(databaseMembers))
		for i, member := range databaseMembers {
			redisMembers[i] = &redis.Member{
				Member: member.Member,
				Score:  member.Score,
			}
		}
		ranks, err := writer.ZAddAndRanks(ctx, leaderboard, order, redisMembers...)
		if err != nil {
			return nil, NewGeneralError(err.Error())
		}
		return ranks, nil
	}

	if err := r.SetMembers(ctx, leaderboard, databaseMembers); err != nil {
		return nil, err
	}
	memberIDs := make([]string, len(databaseMembers))
	for i, member := range databaseMembers {
		memberIDs[i] = member.Member
	}
	members, err := r.GetMembers(ctx, leaderboard, order, false, memberIDs...)
	if err != nil {
		return nil, err
	}
	ranks := make([]int64, len(members))
	for i, member := range members {
		ranks[i] = member.Rank
	}
	return ranks, nil
}

// SetMembersTTL set member ttl in an OrderedSet and add this to expiration_worker set
//
//	The TTL is a different ordered set than the original leaderboard, with key being
//	leaderboard name and suffix ":ttl", for example to a leaderboard named test your
//	orederedset with time to expire will be "test:ttl"
//
//	Note: the worker expiration set is expiration_set
func (r *Redis) SetMembersTTL(ctx context.Context, leaderboard string, databaseMembers []*Member) error {
	redisMembers := make([]*redis.Member, 0, len(databaseMembers))
	for _, member := range databaseMembers {
		redisMembers = append(redisMembers, &redis.Member{
			Member: member.Member,
			Score:  float64(member.TTL.Unix()),
		})
	}

	expirationKey := fmt.Sprintf("%s:ttl", leaderboard)
	err := r.Client.ZAdd(ctx, expirationKey, redisMembers...)
	if err != nil {
		return NewGeneralError(err.Error())
	}

	err = r.Client.SAdd(ctx, ExpirationSet, expirationKey)
	if err != nil {
		return NewGeneralError(err.Error())
	}

	return nil
}
