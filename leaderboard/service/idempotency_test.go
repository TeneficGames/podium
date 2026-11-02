package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/expiration"
	"github.com/TeneficGames/podium/leaderboard/model"
	"github.com/TeneficGames/podium/leaderboard/service"
)

type onceDatabase struct {
	database.Database
	result        *database.Member
	err           error
	calls         int
	ttl, expireAt int64
}

func (d *onceDatabase) IncrementMemberScoreOnce(_ context.Context, board, member string, increment float64, key string, ttl, expireAt int64) (*database.Member, error) {
	d.calls++
	d.ttl = ttl
	d.expireAt = expireAt
	return d.result, d.err
}

type withoutOnceDatabase struct{ database.Database }

func TestIncrementMemberScoreOnce(t *testing.T) {
	ctx := context.Background()
	if _, err := service.NewService(&withoutOnceDatabase{}).IncrementMemberScoreOnce(ctx, "board", "alice", 10, "", "event"); err == nil {
		t.Fatal("unsupported database accepted")
	}
	backendErr := errors.New("backend unavailable")
	for _, tc := range []struct {
		name, board, ttl    string
		backendErr          error
		wantCalls           int
		wantTTL, wantExpiry int64
		wantErr             bool
	}{
		{name: "no ttl", board: "board", wantCalls: 1, wantExpiry: -1},
		{name: "ttl", board: "board", ttl: "60", wantCalls: 1, wantTTL: 60, wantExpiry: -1},
		{name: "invalid ttl", board: "board", ttl: "bad", wantErr: true},
		{name: "invalid season", board: "board-from2000000000to1000000000", wantErr: true},
		{name: "expired season replay", board: "board-from1000000000to1000000001", wantCalls: 1, wantExpiry: -2},
		{name: "expired new operation", board: "board-from1000000000to1000000001", backendErr: &expiration.LeaderboardExpiredError{}, wantCalls: 1, wantExpiry: -2, wantErr: true},
		{name: "conflict", board: "board", backendErr: model.ErrIdempotencyConflict, wantCalls: 1, wantExpiry: -1, wantErr: true},
		{name: "backend failure", board: "board", backendErr: backendErr, wantCalls: 1, wantExpiry: -1, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &onceDatabase{result: &database.Member{Member: "alice", Score: 10, Rank: 2}, err: tc.backendErr}
			if tc.ttl == "60" {
				db.result.TTL = time.Unix(2_000_000_000, 0)
			}
			got, err := service.NewService(db).IncrementMemberScoreOnce(ctx, tc.board, "alice", 10, tc.ttl, "event")
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected result: %+v, %v", got, err)
			}
			if db.calls != tc.wantCalls || db.ttl != tc.wantTTL || db.expireAt != tc.wantExpiry {
				t.Fatalf("unexpected call: %+v", db)
			}
			if errors.Is(tc.backendErr, model.ErrIdempotencyConflict) && !errors.Is(err, model.ErrIdempotencyConflict) {
				t.Fatal("conflict identity lost")
			}
			if errors.Is(tc.backendErr, backendErr) && !errors.Is(err, backendErr) {
				t.Fatal("backend error identity lost")
			}
			var backendExpired *expiration.LeaderboardExpiredError
			if errors.As(tc.backendErr, &backendExpired) {
				var expired *service.LeaderboardExpiredError
				if !errors.As(err, &expired) {
					t.Fatalf("expected service expiry error: %v", err)
				}
			}
			if !tc.wantErr {
				if got.PublicID != "alice" || got.Score != 10 || got.Rank != 3 {
					t.Fatalf("unexpected original response: %+v", got)
				}
				wantTTL := 0
				if tc.ttl == "60" {
					wantTTL = 2_000_000_000
				}
				if got.ExpireAt != wantTTL {
					t.Fatalf("lost response expiry: %+v", got)
				}
			}
		})
	}
}
