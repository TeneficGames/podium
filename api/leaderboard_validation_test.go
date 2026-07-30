package api

import (
	"testing"

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
