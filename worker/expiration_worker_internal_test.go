package worker

import (
	"errors"
	"testing"
	"time"

	"github.com/TeneficGames/podium/leaderboard/database"
	"go.uber.org/mock/gomock"
)

func TestExpireMembersStopsAfterLeaderboardQueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	databaseMock := database.NewMockExpiration(ctrl)
	expectedErr := errors.New("query failed")
	databaseMock.EXPECT().
		GetExpirationLeaderboards(gomock.Any()).
		Return(nil, expectedErr)

	expirationWorker, err := NewExpirationWorker("", 0, "", 0, time.Second, 1)
	if err != nil {
		t.Fatalf("create expiration worker: %v", err)
	}
	expirationWorker.Database = databaseMock

	results := make(chan []*ExpirationResult, 1)
	errs := make(chan error, 1)
	expirationWorker.expireMembers(results, errs)

	if err := <-errs; !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	select {
	case result := <-results:
		t.Fatalf("unexpected expiration result: %#v", result)
	default:
	}
}

func TestStopIsIdempotent(t *testing.T) {
	expirationWorker, err := NewExpirationWorker("", 0, "", 0, time.Second, 1)
	if err != nil {
		t.Fatalf("create expiration worker: %v", err)
	}

	expirationWorker.Stop()
	expirationWorker.Stop()
}
