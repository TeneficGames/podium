package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
)

func addMembersAndGetRanks(
	ctx context.Context,
	client pipelineClient,
	key, order string,
	members ...*Member,
) ([]int64, error) {
	redisMembers := make([]goredis.Z, len(members))
	ranks := make([]*goredis.IntCmd, len(members))

	_, err := client.Pipelined(ctx, func(pipe goredis.Pipeliner) error {
		for i, member := range members {
			redisMembers[i] = goredis.Z{Member: member.Member, Score: member.Score}
		}
		pipe.ZAdd(ctx, key, redisMembers...)
		for i, member := range members {
			if order == "asc" {
				ranks[i] = pipe.ZRank(ctx, key, member.Member)
			} else {
				ranks[i] = pipe.ZRevRank(ctx, key, member.Member)
			}
		}
		return nil
	})
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}

	result := make([]int64, len(ranks))
	for i, rank := range ranks {
		value, err := rank.Result()
		if err != nil {
			return nil, NewGeneralError(err.Error())
		}
		result[i] = value
	}
	return result, nil
}

func incrementMemberAndGetRank(
	ctx context.Context,
	client pipelineClient,
	key, member, order string,
	increment float64,
) (*Member, error) {
	var score *goredis.FloatCmd
	var rank *goredis.IntCmd

	_, err := client.Pipelined(ctx, func(pipe goredis.Pipeliner) error {
		score = pipe.ZIncrBy(ctx, key, increment, member)
		if order == "asc" {
			rank = pipe.ZRank(ctx, key, member)
		} else {
			rank = pipe.ZRevRank(ctx, key, member)
		}
		return nil
	})
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}

	scoreValue, err := score.Result()
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}
	rankValue, err := rank.Result()
	if err != nil {
		return nil, NewGeneralError(err.Error())
	}
	return &Member{
		Member: member,
		Score:  scoreValue,
		Rank:   rankValue,
	}, nil
}
