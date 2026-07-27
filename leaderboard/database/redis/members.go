package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type pipelineClient interface {
	Pipelined(context.Context, func(goredis.Pipeliner) error) ([]goredis.Cmder, error)
}

func getMembers(
	ctx context.Context,
	client pipelineClient,
	key, order string,
	includeTTL bool,
	members ...string,
) ([]*Member, error) {
	if len(members) == 0 {
		return []*Member{}, nil
	}

	rankedScores := make([]*goredis.RankWithScoreCmd, len(members))
	var ttls []*goredis.FloatCmd

	_, err := client.Pipelined(ctx, func(pipe goredis.Pipeliner) error {
		for i, member := range members {
			if order == "asc" {
				rankedScores[i] = pipe.ZRankWithScore(ctx, key, member)
			} else {
				rankedScores[i] = pipe.ZRevRankWithScore(ctx, key, member)
			}
		}
		if includeTTL {
			ttlKey := key + ":ttl"
			ttls = make([]*goredis.FloatCmd, len(members))
			for i, member := range members {
				ttls[i] = pipe.ZScore(ctx, ttlKey, member)
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, goredis.Nil) {
		return nil, NewGeneralError(err.Error())
	}

	result := make([]*Member, len(members))
	for i, member := range members {
		rankedScore, err := rankedScores[i].Result()
		if errors.Is(err, goredis.Nil) {
			continue
		}
		if err != nil {
			return nil, NewGeneralError(err.Error())
		}

		result[i] = &Member{
			Member: member,
			Score:  rankedScore.Score,
			Rank:   rankedScore.Rank,
		}
		if includeTTL {
			ttl, err := ttls[i].Result()
			if err != nil && !errors.Is(err, goredis.Nil) {
				return nil, NewGeneralError(err.Error())
			}
			if err == nil {
				result[i].TTL = time.Unix(int64(ttl), 0)
			}
		}
	}

	return result, nil
}
