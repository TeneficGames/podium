package api_test

import (
	"context"
	"io"
	"net/http"
	"strings"

	pb "github.com/TeneficGames/podium/proto/podium/api/v1"
	. "github.com/TeneficGames/podium/testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var _ = Describe("Idempotent score increments", func() {
	It("shares replay responses between HTTP and gRPC and preserves requests without keys", func() {
		app := GetDefaultTestApp()
		InitializeTestServer(app)
		ctx := context.Background()
		board := "idempotency-api"
		defer func() { Expect(app.Leaderboards.RemoveLeaderboard(ctx, board)).To(Succeed()) }()
		httpIncrement := func(body, key string) (int, string) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPatch, "http://"+app.HTTPEndpoint+"/l/"+board+"/members/alice/score", strings.NewReader(body))
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", key)
			response, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer response.Body.Close()
			data, err := io.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())
			return response.StatusCode, string(data)
		}
		code, first := httpIncrement(`{"increment":100}`, "match-1")
		Expect(code).To(Equal(http.StatusOK))
		code, replay := httpIncrement(`{"increment":100}`, "match-1")
		Expect(code).To(Equal(http.StatusOK))
		Expect(replay).To(Equal(first))
		code, _ = httpIncrement(`{"increment":101}`, "match-1")
		Expect(code).To(Equal(http.StatusConflict))
		SetupGRPC(app, func(client pb.PodiumServiceClient) {
			request := &pb.IncrementScoreRequest{LeaderboardId: board, MemberPublicId: "alice", Body: &pb.IncrementScoreRequest_Body{Increment: 100}}
			keyCtx := metadata.AppendToOutgoingContext(ctx, "idempotency-key", "match-1")
			original, err := client.IncrementScore(keyCtx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(original.Score).To(Equal(float64(100)))
			_, err = client.IncrementScore(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			_, err = client.IncrementScore(ctx, request)
			Expect(err).NotTo(HaveOccurred())
			replayed, err := client.IncrementScore(keyCtx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(proto.Equal(original, replayed)).To(BeTrue())
			request.ScoreTtl = 60
			_, err = client.IncrementScore(keyCtx, request)
			Expect(status.Code(err)).To(Equal(codes.AlreadyExists))
			request.ScoreTtl = 60
			request.Body.Increment = 5
			ttlCtx := metadata.AppendToOutgoingContext(ctx, "idempotency-key", "match-2")
			withTTL, err := client.IncrementScore(ttlCtx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(withTTL.ExpireAt).To(BeNumerically(">", 0))
			replayed, err = client.IncrementScore(ttlCtx, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(proto.Equal(withTTL, replayed)).To(BeTrue())
			member, err := client.GetMember(ctx, &pb.GetMemberRequest{LeaderboardId: board, MemberPublicId: "alice"})
			Expect(err).NotTo(HaveOccurred())
			Expect(member.Score).To(Equal(float64(305)))
		})
	})
})
