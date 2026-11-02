package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/expiration"
	"github.com/TeneficGames/podium/leaderboard/model"
)

type idempotentIncrementer interface {
	IncrementMemberScoreOnce(context.Context, string, string, float64, string, int64, int64) (*database.Member, error)
}

// IncrementMemberScoreOnce returns the original result when the request is retried.
func (s *Service) IncrementMemberScoreOnce(ctx context.Context, leaderboard, member string, increment int, scoreTTL, requestKey string) (*model.Member, error) {
	writer, ok := s.Database.(idempotentIncrementer)
	if !ok {
		return nil, fmt.Errorf("database does not support idempotent increments")
	}
	var ttl int64
	if scoreTTL != "" {
		var err error
		ttl, err = strconv.ParseInt(scoreTTL, 10, 64)
		if err != nil {
			return nil, err
		}
	}
	expireAt, err := expiration.GetExpireAt(leaderboard)
	if err != nil {
		var expired *expiration.LeaderboardExpiredError
		if !errors.As(err, &expired) {
			return nil, err
		}
		expireAt = -2 // A cached response remains replayable after the season expires.
	}
	result, err := writer.IncrementMemberScoreOnce(ctx, leaderboard, member, float64(increment), requestKey, ttl, expireAt)
	if err != nil {
		var expired *expiration.LeaderboardExpiredError
		if errors.As(err, &expired) {
			return nil, NewLeaderboardExpiredError(leaderboard)
		}
		return nil, err
	}
	value := convertDatabaseMemberIntoModelMember(result)
	if !result.TTL.IsZero() {
		value.ExpireAt = int(result.TTL.Unix())
	}
	return value, nil
}
