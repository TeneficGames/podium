package service

import (
	"context"
	"math"
	"time"

	"github.com/TeneficGames/podium/leaderboard/model"
)

const getMemberServiceLabel = "get member"

// GetMember return a member info
func (s *Service) GetMember(ctx context.Context, leaderboard, member string, order string, includeTTL bool) (*model.Member, error) {
	databaseMembers, err := s.Database.GetMembers(ctx, leaderboard, order, includeTTL, member)
	if err != nil {
		return nil, NewGeneralError(getMemberServiceLabel, err.Error())
	}

	if databaseMembers[0] == nil {
		return nil, NewMemberNotFoundError(leaderboard, member)
	}

	if databaseMembers[0].Rank < 0 || databaseMembers[0].Rank >= math.MaxInt32 {
		return nil, NewGeneralError(getMemberServiceLabel, "member rank is outside supported range")
	}

	var ttl int64 = 0
	if (databaseMembers[0].TTL != time.Time{}) {
		ttl = databaseMembers[0].TTL.Unix()
	}

	return &model.Member{
		PublicID: databaseMembers[0].Member,
		Score:    int64(databaseMembers[0].Score),
		Rank:     int(databaseMembers[0].Rank + 1),
		ExpireAt: int(ttl),
	}, nil
}
