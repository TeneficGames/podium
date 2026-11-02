package api_test

import (
	"context"
	"math"
	"net/http"

	"github.com/TeneficGames/podium/leaderboard/model"
	pb "github.com/TeneficGames/podium/proto/podium/api/v1"
	. "github.com/TeneficGames/podium/testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Score input validation", func() {
	It("rejects malformed scores through HTTP and gRPC before any writes", func() {
		app := GetDefaultTestApp()
		InitializeTestServer(app)
		ctx := context.Background()
		leaderboard := "validation-rejected"
		defer func() { Expect(app.Leaderboards.RemoveLeaderboard(ctx, leaderboard)).To(Succeed()) }()

		for _, score := range []any{1.9, -1.9, "NaN", "Infinity", "-Infinity", float64(model.MaxScore + 2)} {
			code, _ := PutJSON(app, "/l/"+leaderboard+"/members/alice/score", map[string]any{"score": score})
			Expect(code).To(Equal(http.StatusBadRequest))
		}
		SetupGRPC(app, func(client pb.PodiumServiceClient) {
			for _, score := range []float64{1.9, math.NaN(), math.Inf(1), math.Inf(-1), model.MaxScore + 2, -model.MaxScore - 2} {
				_, err := client.UpsertScore(ctx, &pb.UpsertScoreRequest{LeaderboardId: leaderboard, MemberPublicId: "alice", ScoreChange: &pb.UpsertScoreRequest_ScoreChange{Score: score}})
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
				_, err = client.IncrementScore(ctx, &pb.IncrementScoreRequest{LeaderboardId: leaderboard, MemberPublicId: "alice", Body: &pb.IncrementScoreRequest_Body{Increment: score}})
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
				_, err = client.UpsertScoreMultiLeaderboards(ctx, &pb.UpsertScoreMultiLeaderboardsRequest{MemberPublicId: "alice", ScoreMultiChange: &pb.UpsertScoreMultiLeaderboardsRequest_ScoreMultiChange{Score: score, Leaderboards: []string{leaderboard}}})
				Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			}
			_, err := client.UpsertScore(ctx, &pb.UpsertScoreRequest{LeaderboardId: leaderboard})
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			_, err = client.IncrementScore(ctx, &pb.IncrementScoreRequest{LeaderboardId: leaderboard})
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			_, err = client.UpsertScoreMultiLeaderboards(ctx, &pb.UpsertScoreMultiLeaderboardsRequest{})
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			_, err = client.BulkUpsertScores(ctx, &pb.BulkUpsertScoresRequest{LeaderboardId: leaderboard, MemberScores: &pb.BulkUpsertScoresRequest_MemberScores{Members: []*pb.BulkUpsertScoresRequest_MemberScore{{PublicId: "valid", Score: 10}, {PublicId: "invalid", Score: 1.9}}}})
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
		})
		code, _ := PutJSON(app, "/l/"+leaderboard+"/scores", map[string]any{"members": []map[string]any{{"publicId": "valid", "score": 10}, {"publicId": "invalid", "score": 1.9}}})
		Expect(code).To(Equal(http.StatusBadRequest))
		count, err := app.Leaderboards.TotalMembers(ctx, leaderboard)
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(BeZero())
	})

	It("accepts boundaries and rejects overflowing increments without changing scores", func() {
		app := GetDefaultTestApp()
		ctx := context.Background()
		leaderboard := "validation-boundaries"
		defer func() { Expect(app.Leaderboards.RemoveLeaderboard(ctx, leaderboard)).To(Succeed()) }()
		SetupGRPC(app, func(client pb.PodiumServiceClient) {
			for _, score := range []float64{model.MaxScore, -model.MaxScore, 0, -10} {
				_, err := client.UpsertScore(ctx, &pb.UpsertScoreRequest{LeaderboardId: leaderboard, MemberPublicId: "alice", ScoreChange: &pb.UpsertScoreRequest_ScoreChange{Score: score}})
				Expect(err).NotTo(HaveOccurred())
				if math.Abs(score) == model.MaxScore {
					increment := math.Copysign(1, score)
					_, err = client.IncrementScore(ctx, &pb.IncrementScoreRequest{LeaderboardId: leaderboard, MemberPublicId: "alice", Body: &pb.IncrementScoreRequest_Body{Increment: increment}})
					Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
				}
				member, err := client.GetMember(ctx, &pb.GetMemberRequest{LeaderboardId: leaderboard, MemberPublicId: "alice"})
				Expect(err).NotTo(HaveOccurred())
				Expect(member.Score).To(Equal(score))
			}
		})
	})
})
