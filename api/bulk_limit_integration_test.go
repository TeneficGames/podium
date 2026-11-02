package api_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/TeneficGames/podium/api"
	pb "github.com/TeneficGames/podium/proto/podium/api/v1"
	. "github.com/TeneficGames/podium/testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Bulk member limit", func() {
	It("accepts boundary batches and rejects oversized HTTP/gRPC batches without writes", func() {
		app := GetDefaultTestApp()
		InitializeTestServer(app)
		ctx := context.Background()
		board := "bulk-member-limit"
		defer func() { Expect(app.Leaderboards.RemoveLeaderboard(ctx, board)).To(Succeed()) }()
		request := func(count int, score float64) *pb.BulkUpsertScoresRequest {
			members := make([]*pb.BulkUpsertScoresRequest_MemberScore, count)
			for i := range members {
				members[i] = &pb.BulkUpsertScoresRequest_MemberScore{PublicId: fmt.Sprintf("player-%d", i), Score: score}
			}
			return &pb.BulkUpsertScoresRequest{LeaderboardId: board, PrevRank: true, ScoreTtl: 60, MemberScores: &pb.BulkUpsertScoresRequest_MemberScores{Members: members}}
		}
		var bare api.App
		_, err := bare.BulkUpsertScores(ctx, request(1001, 200))
		Expect(status.Code(err)).To(Equal(codes.InvalidArgument)) // Rejection needs no logger or database.
		SetupGRPC(app, func(client pb.PodiumServiceClient) {
			_, err := client.BulkUpsertScores(ctx, request(1001, 200))
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
			count, err := app.Leaderboards.TotalMembers(ctx, board)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(BeZero())
			for _, size := range []int{999, 1000} {
				response, err := client.BulkUpsertScores(ctx, request(size, 100))
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Members).To(HaveLen(size))
				Expect(response.Members[0].Rank).To(Equal(int32(1)))
				Expect(response.Members[size-1].Rank).To(Equal(int32(size)))
				Expect(response.Members[size-1].PreviousRank).To(Equal(int32(-1)))
				Expect(response.Members[0].ExpireAt).To(BeNumerically(">", 0))
			}
			_, err = client.BulkUpsertScores(ctx, request(1001, 200))
			Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
		})
		members := make([]map[string]any, 1001)
		for i := range members {
			members[i] = map[string]any{"publicId": fmt.Sprintf("player-%d", i), "score": 200}
		}
		code, _ := PutJSON(app, "/l/"+board+"/scores", map[string]any{"members": members})
		Expect(code).To(Equal(http.StatusBadRequest))
		count, err := app.Leaderboards.TotalMembers(ctx, board)
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(Equal(1000))
		member, err := app.Leaderboards.GetMember(ctx, board, "player-0", "desc", true)
		Expect(err).NotTo(HaveOccurred())
		Expect(member.Score).To(Equal(int64(100)))
	})
})
