package api

import (
	"context"
	"testing"

	"github.com/TeneficGames/podium/leaderboard/service"
	pb "github.com/TeneficGames/podium/proto/podium/api/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type leaderboardWithoutIdempotency struct{ service.Leaderboard }

func TestIdempotencyRequiresSupportedLeaderboard(t *testing.T) {
	app := &App{Logger: zap.NewNop(), Leaderboards: &leaderboardWithoutIdempotency{}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(IdempotencyKeyHeader, "match-1"))
	result, err := app.IncrementScore(ctx, &pb.IncrementScoreRequest{
		LeaderboardId: "board", MemberPublicId: "alice",
		Body: &pb.IncrementScoreRequest_Body{Increment: 10},
	})
	if result != nil || status.Code(err) != codes.Unimplemented {
		t.Fatalf("unsupported backend must reject instead of incrementing without deduplication: %+v, %v", result, err)
	}
}
