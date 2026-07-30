package database

import (
	"strings"
	"testing"
)

func TestLeaderboardKeysAreUniqueAndColocated(t *testing.T) {
	first := leaderboardKeys("a:{shared}:leaderboard")
	second := leaderboardKeys("different")
	firstKeys := []string{first.Scores, first.ScoresAsc, first.Members, first.Sequence, first.TTL}

	tag := redisHashTag(t, first.Scores)
	if tag == "" {
		t.Fatal("leaderboard key has an empty Redis Cluster hash tag")
	}
	for _, key := range firstKeys[1:] {
		if redisHashTag(t, key) != tag {
			t.Fatalf("keys are not colocated: %v", firstKeys)
		}
	}
	if redisHashTag(t, second.Scores) == tag {
		t.Fatal("different leaderboard IDs generated the same hash tag")
	}
}

func TestDecodeTieBreakMember(t *testing.T) {
	for _, publicID := range []string{"", "ordinary", "玩家", "contains:{braces}", "contains\x00null"} {
		encoded := "9223372036854775806" + publicID
		decoded, err := decodeTieBreakMember(encoded)
		if err != nil {
			t.Fatalf("decode %q: %v", publicID, err)
		}
		if decoded != publicID {
			t.Fatalf("decode %q: got %q", publicID, decoded)
		}
	}

	for _, malformed := range []string{"", "123", "x223372036854775806member"} {
		if _, err := decodeTieBreakMember(malformed); err == nil {
			t.Fatalf("expected %q to be rejected", malformed)
		}
	}
}

func TestValidateDistinctMembers(t *testing.T) {
	if err := validateDistinctMembers([]*Member{{Member: "a"}, {Member: "b"}}); err != nil {
		t.Fatal(err)
	}
	if err := validateDistinctMembers([]*Member{{Member: "a"}, {Member: "a"}}); err == nil {
		t.Fatal("expected duplicate members to be rejected")
	}
	if err := validateDistinctMembers([]*Member{nil}); err == nil {
		t.Fatal("expected nil members to be rejected")
	}
}

func redisHashTag(t *testing.T, key string) string {
	t.Helper()
	start := strings.IndexByte(key, '{')
	end := strings.IndexByte(key, '}')
	if start == -1 || end <= start+1 {
		t.Fatalf("key %q does not contain a non-empty hash tag", key)
	}
	return key[start+1 : end]
}
