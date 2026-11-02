package api

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/metadata"

	pb "github.com/TeneficGames/podium/proto/podium/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidateBulkUpsertScoresRequest(t *testing.T) {
	tests := []struct {
		name    string
		request *pb.BulkUpsertScoresRequest
		message string
	}{
		{
			name:    "missing member scores",
			request: &pb.BulkUpsertScoresRequest{},
			message: "memberScores is required",
		},
		{
			name: "empty members",
			request: &pb.BulkUpsertScoresRequest{
				MemberScores: &pb.BulkUpsertScoresRequest_MemberScores{},
			},
			message: "at least one member is required",
		},
		{
			name: "nil member",
			request: &pb.BulkUpsertScoresRequest{
				MemberScores: &pb.BulkUpsertScoresRequest_MemberScores{
					Members: []*pb.BulkUpsertScoresRequest_MemberScore{nil},
				},
			},
			message: "member is required",
		},
		{
			name: "duplicate member",
			request: &pb.BulkUpsertScoresRequest{
				MemberScores: &pb.BulkUpsertScoresRequest_MemberScores{
					Members: []*pb.BulkUpsertScoresRequest_MemberScore{
						{PublicId: "same"},
						{PublicId: "same"},
					},
				},
			},
			message: "duplicate publicId: same",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateBulkUpsertScoresRequest(test.request)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("expected InvalidArgument, got %v", err)
			}
			if status.Convert(err).Message() != test.message {
				t.Fatalf("expected %q, got %q", test.message, status.Convert(err).Message())
			}
		})
	}
}

func TestMissingScoreMessages(t *testing.T) {
	app := &App{}
	ctx := context.Background()
	checks := []struct {
		name string
		call func() error
	}{
		{"single", func() error { _, err := app.UpsertScore(ctx, &pb.UpsertScoreRequest{}); return err }},
		{"increment", func() error { _, err := app.IncrementScore(ctx, &pb.IncrementScoreRequest{}); return err }},
		{"multi", func() error {
			_, err := app.UpsertScoreMultiLeaderboards(ctx, &pb.UpsertScoreMultiLeaderboardsRequest{})
			return err
		}},
		{"bulk", func() error { _, err := app.BulkUpsertScores(ctx, &pb.BulkUpsertScoresRequest{}); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.call(); status.Code(err) != codes.InvalidArgument {
				t.Fatalf("expected InvalidArgument, got %v", err)
			}
		})
	}
}

func TestInvalidIdempotencyKeys(t *testing.T) {
	app := &App{}
	for _, keys := range [][]string{{""}, {"   "}, {strings.Repeat("k", 257)}, {"one", "two"}} {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{IdempotencyKeyHeader: keys})
		_, err := app.IncrementScore(ctx, &pb.IncrementScoreRequest{Body: &pb.IncrementScoreRequest_Body{Increment: 1}})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("invalid keys accepted: %v", err)
		}
	}
}

func TestBulkUpsertMemberLimit(t *testing.T) {
	for _, count := range []int{0, 999, 1000, 1001} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			members := make([]*pb.BulkUpsertScoresRequest_MemberScore, count)
			for i := range members {
				members[i] = &pb.BulkUpsertScoresRequest_MemberScore{PublicId: fmt.Sprintf("player-%d", i), Score: 100}
			}
			req := &pb.BulkUpsertScoresRequest{MemberScores: &pb.BulkUpsertScoresRequest_MemberScores{Members: members}}
			err := validateBulkUpsertScoresRequest(req)
			want := codes.OK
			if count == 0 || count > 1000 {
				want = codes.InvalidArgument
			}
			if status.Code(err) != want {
				t.Fatalf("count %d: expected %s, got %v", count, want, err)
			}
		})
	}
}
