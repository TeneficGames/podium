// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package database

import "testing"

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "general",
			err:  NewGeneralError("connection lost"),
			want: "database error: connection lost",
		},
		{
			name: "invalid order",
			err:  NewInvalidOrderError("sideways"),
			want: "invalid order: sideways",
		},
		{
			name: "member not found",
			err:  NewMemberNotFoundError("weekly", "player-1"),
			want: "member player-1 not found in leaderboard weekly",
		},
		{
			name: "TTL not found",
			err:  NewTTLNotFoundError("weekly"),
			want: "ttl to leaderboard weekly not found",
		},
		{
			name: "no member to expire",
			err:  NewLeaderboardWithoutMemberToExpireError("weekly"),
			want: "leaderboard weekly without member to expire",
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
