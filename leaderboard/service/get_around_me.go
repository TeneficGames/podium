package service

import (
	"context"
	"errors"

	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/model"
)

const getAroundMeServiceLabel = "get around me"

// GetAroundMe find users around a certain member
func (s *Service) GetAroundMe(ctx context.Context, leaderboard string, pageSize int, member string, order string, getLastIfNotFound bool) ([]*model.Member, error) {
	memberRank, err := s.fetchMemberRank(ctx, leaderboard, member, order, getLastIfNotFound)
	if err != nil {
		var notFoundErr *database.MemberNotFoundError
		if errors.As(err, &notFoundErr) {
			return nil, NewMemberNotFoundError(leaderboard, member)
		}

		return nil, NewGeneralError(getAroundMeServiceLabel, err.Error())
	}

	indexes, err := s.calculateIndexesAroundMemberRank(ctx, leaderboard, memberRank, pageSize)
	if err != nil {
		return nil, NewGeneralError(getAroundMeServiceLabel, err.Error())
	}

	databaseMembers, err := s.Database.GetOrderedMembers(ctx, leaderboard, indexes.Start, indexes.Stop, order)
	if err != nil {
		return nil, NewGeneralError(getAroundMeServiceLabel, err.Error())
	}

	members := convertDatabaseMembersIntoModelMembers(databaseMembers)

	return members, nil
}
