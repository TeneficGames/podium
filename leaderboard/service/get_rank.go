package service

import (
	"context"
	"errors"

	"github.com/TeneficGames/podium/leaderboard/database"
)

const getRankServiceLabel = "get rank"

// GetRank return the current member rank in a specific order
func (s *Service) GetRank(ctx context.Context, leaderboard, member, order string) (int, error) {
	rank, err := s.Database.GetRank(ctx, leaderboard, member, order)
	if err != nil {
		var notFoundErr *database.MemberNotFoundError
		if errors.As(err, &notFoundErr) {
			return -1, NewMemberNotFoundError(leaderboard, member)
		}

		return -1, NewGeneralError(getRankServiceLabel, err.Error())
	}

	return rank + 1, nil
}
