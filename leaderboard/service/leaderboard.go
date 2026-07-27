package service

import (
	"context"
	"errors"
	"time"

	"github.com/TeneficGames/podium/leaderboard/v2/database"
	"github.com/TeneficGames/podium/leaderboard/v2/expiration"
)

func (s *Service) persistLeaderboardExpirationTime(ctx context.Context, leaderboard string) error {
	expireAt, err := expiration.GetExpireAt(leaderboard)
	if err != nil {
		return err
	}

	if expireAt == -1 {
		return nil
	}

	_, err = s.Database.GetLeaderboardExpiration(ctx, leaderboard)
	if err != nil {
		var notFoundErr *database.TTLNotFoundError
		if errors.As(err, &notFoundErr) {
			err = s.Database.SetLeaderboardExpiration(ctx, leaderboard, time.Unix(expireAt, 0))
			if err != nil {
				return err
			}

		} else {
			return err
		}
	}

	return nil
}
