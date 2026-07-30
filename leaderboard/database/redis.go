package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	podiumredis "github.com/TeneficGames/podium/leaderboard/database/redis"
)

// Redis is a type that implements Database interface with redis client
type Redis struct {
	podiumredis.Client
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
		return &Redis{podiumredis.NewClusterClient(podiumredis.ClusterOptions{
			Addrs:    options.Addrs,
			Password: options.Password,
		})}
	}

	return &Redis{podiumredis.NewStandaloneClient(podiumredis.StandaloneOptions{
		Host:     options.Host,
		Port:     options.Port,
		Password: options.Password,
		DB:       options.DB,
	})}
}

// GetLeaderboardExpiration return leaderboard expiration time
func (r *Redis) GetLeaderboardExpiration(ctx context.Context, leaderboard string) (int64, error) {
	key := leaderboard
	if _, ok := r.tieBreakStore(); ok {
		key = leaderboardKeys(leaderboard).Scores
	}
	duration, err := r.Client.TTL(ctx, key)
	if err != nil {
		var notFoundErr *podiumredis.TTLNotFoundError
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

	if store, ok := r.tieBreakStore(); ok {
		redisMembers, err := store.GetMembersWithTieBreak(
			ctx,
			leaderboardKeys(leaderboard),
			order,
			includeTTL,
			members...,
		)
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

	if reader, ok := r.Client.(podiumredis.MemberReader); ok {
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
			var notFoundErr *podiumredis.MemberNotFoundError
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
	if _, ok := r.tieBreakStore(); ok {
		leaderboardTTL = leaderboardKeys(leaderboard).TTL
	}
	ttl, err := r.Client.ZScore(ctx, leaderboardTTL, member)
	if err != nil {
		var notFoundErr *podiumredis.MemberNotFoundError
		if errors.As(err, &notFoundErr) {
			return time.Time{}, NewMemberNotFoundError(leaderboardTTL, member)
		}
		return time.Time{}, NewGeneralError(err.Error())
	}

	return time.Unix(int64(ttl), 0), nil
}

// GetMemberIDsWithScoreInsideRange find members with score close to
func (r *Redis) GetMemberIDsWithScoreInsideRange(ctx context.Context, leaderboard string, min, max string, offset, count int) ([]string, error) {
	key := leaderboard
	tieBreak := false
	if _, ok := r.tieBreakStore(); ok {
		key = leaderboardKeys(leaderboard).Scores
		tieBreak = true
	}
	members, err := r.Client.ZRevRangeByScore(ctx, key, min, max, int64(offset), int64(count))
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}

	if tieBreak {
		for i, member := range members {
			publicID, err := decodeTieBreakMember(member)
			if err != nil {
				return nil, NewGeneralError(err.Error())
			}
			members[i] = publicID
		}
	}
	return members, nil
}

// GetOrderedMembers call redis ZRange if order is asc, if desc call redis ZRevRange
func (r *Redis) GetOrderedMembers(ctx context.Context, leaderboard string, start, stop int, order string) ([]*Member, error) {
	var redisMembers []*podiumredis.Member
	var err error
	key := leaderboard
	tieBreak := false
	if _, ok := r.tieBreakStore(); ok {
		keys := leaderboardKeys(leaderboard)
		if order == "asc" {
			key = keys.ScoresAsc
		} else {
			key = keys.Scores
		}
		tieBreak = true
	}

	switch order {
	case "asc":
		redisMembers, err = r.Client.ZRange(ctx, key, int64(start), int64(stop))
	case "desc":
		redisMembers, err = r.Client.ZRevRange(ctx, key, int64(start), int64(stop))
	default:
		return nil, NewInvalidOrderError(order)
	}

	if err != nil {
		return nil, NewGeneralError(err.Error())
	}

	members := make([]*Member, 0, len(redisMembers))
	for i, member := range redisMembers {
		publicID := member.Member
		if tieBreak {
			publicID, err = decodeTieBreakMember(member.Member)
			if err != nil {
				return nil, NewGeneralError(err.Error())
			}
		}
		members = append(members, &Member{
			Member: publicID,
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

	if order != "asc" && order != "desc" {
		return -1, NewInvalidOrderError(order)
	}
	if store, ok := r.tieBreakStore(); ok {
		rank, err = store.GetRankWithTieBreak(ctx, leaderboardKeys(leaderboard), member, order)
		if err != nil {
			var notFoundErr *podiumredis.MemberNotFoundError
			if errors.As(err, &notFoundErr) {
				return -1, NewMemberNotFoundError(leaderboard, member)
			}
			return -1, NewGeneralError(err.Error())
		}
		return int(rank), nil
	}

	switch order {
	case "asc":
		rank, err = r.Client.ZRank(ctx, leaderboard, member)
	case "desc":
		rank, err = r.Client.ZRevRank(ctx, leaderboard, member)
	default:
		return -1, NewInvalidOrderError(order)
	}

	if err != nil {
		var notFoundErr *podiumredis.MemberNotFoundError
		if errors.As(err, &notFoundErr) {
			return -1, NewMemberNotFoundError(leaderboard, member)
		}

		return -1, NewGeneralError(err.Error())
	}

	return int(rank), nil
}

// GetTotalMembers return total members in a leaderboard
func (r *Redis) GetTotalMembers(ctx context.Context, leaderboard string) (int, error) {
	key := leaderboard
	if _, ok := r.tieBreakStore(); ok {
		key = leaderboardKeys(leaderboard).Scores
	}
	totalMembers, err := r.Client.ZCard(ctx, key)
	if err != nil {
		var notFoundErr *podiumredis.KeyNotFoundError
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
	if store, ok := r.tieBreakStore(); ok {
		_, err := store.IncrementWithTieBreak(
			ctx,
			leaderboardKeys(leaderboard),
			member,
			"desc",
			increment,
		)
		if err != nil {
			return NewGeneralError(err.Error())
		}
		return nil
	}
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
	if order != "asc" && order != "desc" {
		return nil, NewInvalidOrderError(order)
	}
	if store, ok := r.tieBreakStore(); ok {
		redisMember, err := store.IncrementWithTieBreak(
			ctx,
			leaderboardKeys(leaderboard),
			member,
			order,
			increment,
		)
		if err != nil {
			return nil, NewGeneralError(err.Error())
		}
		return &Member{
			Member: redisMember.Member,
			Score:  redisMember.Score,
			Rank:   redisMember.Rank,
		}, nil
	}

	if incrementer, ok := r.Client.(podiumredis.MemberIncrementer); ok {
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
	if store, ok := r.tieBreakStore(); ok {
		if err := store.DeleteLeaderboard(ctx, leaderboardKeys(leaderboard)); err != nil {
			return NewGeneralError(err.Error())
		}
		if err := r.Client.SRem(ctx, ExpirationSet, leaderboard); err != nil {
			return NewGeneralError(err.Error())
		}
		return nil
	}
	err := r.Client.Del(ctx, leaderboard)
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// RemoveMembers delete from redis members
func (r *Redis) RemoveMembers(ctx context.Context, leaderboard string, members ...string) error {
	if store, ok := r.tieBreakStore(); ok {
		if err := store.ExpireMembersWithTieBreak(ctx, leaderboardKeys(leaderboard), members...); err != nil {
			return NewGeneralError(err.Error())
		}
		return nil
	}
	err := r.Client.ZRem(ctx, leaderboard, members...)
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// SetLeaderboardExpiration will set leaderboard expiration time
func (r *Redis) SetLeaderboardExpiration(ctx context.Context, leaderboard string, expireAt time.Time) error {
	if store, ok := r.tieBreakStore(); ok {
		if err := store.ExpireTieBreakKeysAt(ctx, leaderboardKeys(leaderboard), expireAt); err != nil {
			return NewGeneralError(err.Error())
		}
		return nil
	}
	err := r.Client.ExpireAt(ctx, leaderboard, expireAt)
	if err != nil {
		return NewGeneralError(err.Error())
	}
	return nil
}

// SetMembers will set member score and ttl
func (r *Redis) SetMembers(ctx context.Context, leaderboard string, databaseMembers []*Member) error {
	if err := validateDistinctMembers(databaseMembers); err != nil {
		return err
	}
	if store, ok := r.tieBreakStore(); ok {
		redisMembers := make([]*podiumredis.Member, len(databaseMembers))
		for i, member := range databaseMembers {
			redisMembers[i] = &podiumredis.Member{Member: member.Member, Score: member.Score}
		}
		if _, err := store.UpsertMembersWithTieBreak(
			ctx,
			leaderboardKeys(leaderboard),
			"desc",
			redisMembers...,
		); err != nil {
			return NewGeneralError(err.Error())
		}
		return nil
	}
	redisMembers := make([]*podiumredis.Member, 0, len(databaseMembers))
	for _, member := range databaseMembers {
		redisMembers = append(redisMembers, &podiumredis.Member{
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
	if order != "asc" && order != "desc" {
		return nil, NewInvalidOrderError(order)
	}
	if err := validateDistinctMembers(databaseMembers); err != nil {
		return nil, err
	}
	if store, ok := r.tieBreakStore(); ok {
		redisMembers := make([]*podiumredis.Member, len(databaseMembers))
		for i, member := range databaseMembers {
			redisMembers[i] = &podiumredis.Member{Member: member.Member, Score: member.Score}
		}
		ranks, err := store.UpsertMembersWithTieBreak(
			ctx,
			leaderboardKeys(leaderboard),
			order,
			redisMembers...,
		)
		if err != nil {
			return nil, NewGeneralError(err.Error())
		}
		return ranks, nil
	}

	if writer, ok := r.Client.(podiumredis.MemberWriter); ok {
		redisMembers := make([]*podiumredis.Member, len(databaseMembers))
		for i, member := range databaseMembers {
			redisMembers[i] = &podiumredis.Member{
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
	if store, ok := r.tieBreakStore(); ok {
		redisMembers := make([]*podiumredis.Member, len(databaseMembers))
		for i, member := range databaseMembers {
			redisMembers[i] = &podiumredis.Member{Member: member.Member, TTL: member.TTL}
		}
		if err := store.SetMembersTTLWithTieBreak(
			ctx,
			leaderboardKeys(leaderboard),
			redisMembers...,
		); err != nil {
			return NewGeneralError(err.Error())
		}
		if err := r.Client.SAdd(ctx, ExpirationSet, leaderboard); err != nil {
			return NewGeneralError(err.Error())
		}
		return nil
	}
	redisMembers := make([]*podiumredis.Member, 0, len(databaseMembers))
	for _, member := range databaseMembers {
		redisMembers = append(redisMembers, &podiumredis.Member{
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

func validateDistinctMembers(members []*Member) error {
	seen := make(map[string]struct{}, len(members))
	for _, member := range members {
		if member == nil {
			return NewGeneralError("member is nil")
		}
		if _, ok := seen[member.Member]; ok {
			return NewGeneralError(fmt.Sprintf("duplicate member %s", member.Member))
		}
		seen[member.Member] = struct{}{}
	}
	return nil
}
