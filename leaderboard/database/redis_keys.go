package database

import (
	"encoding/base64"
	"fmt"

	podiumredis "github.com/TeneficGames/podium/leaderboard/database/redis"
)

const tieBreakSequenceWidth = 19

func leaderboardKeys(leaderboard string) podiumredis.TieBreakKeys {
	tag := "b" + base64.RawURLEncoding.EncodeToString([]byte(leaderboard))
	prefix := fmt.Sprintf("podium:{%s}", tag)
	return podiumredis.TieBreakKeys{
		Scores:    prefix + ":scores",
		ScoresAsc: prefix + ":scores-asc",
		Members:   prefix + ":members",
		Sequence:  prefix + ":sequence",
		TTL:       prefix + ":ttl",
	}
}

func decodeTieBreakMember(internalMember string) (string, error) {
	if len(internalMember) < tieBreakSequenceWidth {
		return "", fmt.Errorf("invalid encoded leaderboard member")
	}
	for i := 0; i < tieBreakSequenceWidth; i++ {
		if internalMember[i] < '0' || internalMember[i] > '9' {
			return "", fmt.Errorf("invalid encoded leaderboard member")
		}
	}
	return internalMember[tieBreakSequenceWidth:], nil
}

func (r *Redis) tieBreakStore() (podiumredis.TieBreakStore, bool) {
	store, ok := r.Client.(podiumredis.TieBreakStore)
	return store, ok
}
