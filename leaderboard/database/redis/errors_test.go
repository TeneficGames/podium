// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package redis

import "testing"

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "key not found",
			err:  NewKeyNotFoundError("weekly"),
			want: "redis: key weekly not found",
		},
		{
			name: "TTL not found",
			err:  NewTTLNotFoundError("weekly"),
			want: "redis: ttl to key weekly not found",
		},
		{
			name: "member not found",
			err:  NewMemberNotFoundError("weekly", "player-1"),
			want: "redis: key weekly does not contain member player-1",
		},
		{
			name: "general",
			err:  NewGeneralError("connection lost"),
			want: "redis: error: connection lost",
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
