// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package expiration

import "fmt"

// InvalidDurationError identifies that a given key generates an invalid duration
type InvalidDurationError struct {
	LeaderboardPublicID string
	DurationInSeconds   int64
}

func (e *InvalidDurationError) Error() string {
	return fmt.Sprintf("Leaderboard %s has invalid duration %v", e.LeaderboardPublicID, e.DurationInSeconds)
}

// LeaderboardExpiredError identifies that a given key generates an already expired leaderboard
type LeaderboardExpiredError struct {
	LeaderboardPublicID string
}

func (e *LeaderboardExpiredError) Error() string {
	return fmt.Sprintf("Leaderboard %s has already expired", e.LeaderboardPublicID)
}
