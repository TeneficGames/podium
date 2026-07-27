// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package expiration

import "testing"

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "invalid duration",
			err: &InvalidDurationError{
				LeaderboardPublicID: "weekly",
				DurationInSeconds:   -1,
			},
			want: "Leaderboard weekly has invalid duration -1",
		},
		{
			name: "leaderboard expired",
			err: &LeaderboardExpiredError{
				LeaderboardPublicID: "weekly",
			},
			want: "Leaderboard weekly has already expired",
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
