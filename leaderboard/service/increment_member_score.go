package service

import (
	"context"
	"errors"

	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/expiration"
	"github.com/TeneficGames/podium/leaderboard/model"
)

const incrementMemberScoreServiceLabel = "increment member score"

const incrementMemberOrder = "desc"

// IncrementMemberScore return member informations that had you score incremented
func (s *Service) IncrementMemberScore(ctx context.Context, leaderboard string, member string, increment int, scoreTTL string) (*model.Member, error) {
	modelMember := &model.Member{
		PublicID: member,
		Score:    int64(increment),
	}

	err := s.incrementMemberAndSetValues(ctx, leaderboard, modelMember, incrementMemberOrder, increment)
	if err != nil {
		return nil, NewGeneralError(incrementMemberScoreServiceLabel, err.Error())
	}

	members := []*model.Member{modelMember}

	err = s.persistLeaderboardExpirationTime(ctx, leaderboard)
	if err != nil {
		var expiredErr *expiration.LeaderboardExpiredError
		if errors.As(err, &expiredErr) {
			return nil, NewLeaderboardExpiredError(leaderboard)
		}
		return nil, NewGeneralError(incrementMemberScoreServiceLabel, err.Error())
	}

	if scoreTTL != "" {
		err = s.persistMembersTTL(ctx, leaderboard, members, scoreTTL)
		if err != nil {
			return nil, NewGeneralError(incrementMemberScoreServiceLabel, err.Error())
		}
	}

	return modelMember, nil
}

func (s *Service) incrementMember(ctx context.Context, leaderboard, member string, increment int) error {
	return s.Database.IncrementMemberScore(ctx, leaderboard, member, float64(increment))
}

type memberIncrementer interface {
	IncrementMemberScoreAndGetRank(
		ctx context.Context,
		leaderboard, member, order string,
		increment float64,
	) (*database.Member, error)
}

func (s *Service) incrementMemberAndSetValues(
	ctx context.Context,
	leaderboard string,
	member *model.Member,
	order string,
	increment int,
) error {
	incrementer, ok := s.Database.(memberIncrementer)
	if !ok {
		if err := s.incrementMember(ctx, leaderboard, member.PublicID, increment); err != nil {
			return err
		}
		return s.setMembersValues(ctx, leaderboard, []*model.Member{member}, order)
	}

	databaseMember, err := incrementer.IncrementMemberScoreAndGetRank(
		ctx,
		leaderboard,
		member.PublicID,
		order,
		float64(increment),
	)
	if err != nil {
		return err
	}
	member.Score = int64(databaseMember.Score)
	member.Rank = int(databaseMember.Rank + 1)
	return nil
}
