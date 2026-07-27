// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package service

import "testing"

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "general",
			err:  NewGeneralError("get member", "database unavailable"),
			want: "error on service get member: database unavailable",
		},
		{
			name: "member not found",
			err:  NewMemberNotFoundError("weekly", "player-1"),
			want: "Could not find data for member player-1 in leaderboard weekly.",
		},
		{
			name: "page out of range",
			err:  NewPageOutOfRangeError(3, 2),
			want: "page 3 out of range (1, 2)",
		},
		{
			name: "leaderboard expired",
			err:  NewLeaderboardExpiredError("weekly"),
			want: "Leaderboard expired error: weekly",
		},
		{
			name: "invalid percentage",
			err:  NewPercentageError(101),
			want: "percentage error: 101, it must be a valid integer between 1 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
