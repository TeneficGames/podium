package client

import (
	"context"
	"net"
	"reflect"
	"testing"

	api "github.com/TeneficGames/podium/proto/podium/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

func TestGRPCClient(t *testing.T) {
	service := &recordingPodiumServer{}
	client := newTestGRPCClient(t, service)

	t.Run("healthcheck", func(t *testing.T) {
		got, err := client.Healthcheck(nil)
		requireNoError(t, err)
		requireEqual(t, got, "WORKING")
	})

	t.Run("delete leaderboard", func(t *testing.T) {
		got, err := client.DeleteLeaderboard(context.Background(), "weekly")
		requireNoError(t, err)
		requireEqual(t, got, &Response{Success: true, Reason: "deleted"})
		requireProtoEqual(t, service.removeLeaderboardRequest, &api.RemoveLeaderboardRequest{
			LeaderboardId: "weekly",
		})
	})

	t.Run("get count", func(t *testing.T) {
		got, err := client.GetCount(context.Background(), "weekly")
		requireNoError(t, err)
		requireEqual(t, got, 42)
		requireProtoEqual(t, service.totalMembersRequest, &api.TotalMembersRequest{
			LeaderboardId: "weekly",
		})
	})

	t.Run("get member", func(t *testing.T) {
		got, err := client.GetMember(context.Background(), "weekly", "player-1")
		requireNoError(t, err)
		requireEqual(t, got, &Member{
			LeaderboardID: "weekly",
			PublicID:      "player-1",
			Score:         120,
			Rank:          2,
			PreviousRank:  3,
		})
		requireProtoEqual(t, service.getMemberRequest, &api.GetMemberRequest{
			LeaderboardId:  "weekly",
			MemberPublicId: "player-1",
		})
	})

	t.Run("get members", func(t *testing.T) {
		got, err := client.GetMembers(context.Background(), "weekly", []string{"player-1", "missing"})
		requireNoError(t, err)
		requireEqual(t, got, &MemberList{
			Members: []*Member{{
				LeaderboardID: "weekly",
				PublicID:      "player-1",
				Score:         120,
				Rank:          2,
			}},
			NotFound: []string{"missing"},
		})
		requireProtoEqual(t, service.getMembersRequest, &api.GetMembersRequest{
			LeaderboardId: "weekly",
			Ids:           "player-1,missing",
		})
	})

	t.Run("get member in leaderboards", func(t *testing.T) {
		got, err := client.GetMemberInLeaderboards(
			context.Background(),
			[]string{"weekly", "monthly"},
			"player-1",
		)
		requireNoError(t, err)
		requireEqual(t, got, &ScoreList{Scores: []*Score{{
			LeaderboardID: "weekly",
			PublicID:      "player-1",
			Score:         120,
			Rank:          2,
		}}})
		requireProtoEqual(t, service.getRankMultiLeaderboardsRequest, &api.GetRankMultiLeaderboardsRequest{
			MemberPublicId: "player-1",
			LeaderboardIds: "weekly,monthly",
			Order:          "desc",
		})
	})

	t.Run("get members around member", func(t *testing.T) {
		got, err := client.GetMembersAroundMember(
			context.Background(),
			"weekly",
			"player-1",
			5,
			true,
			"asc",
		)
		requireNoError(t, err)
		requireEqual(t, got, &MemberList{Members: []*Member{{
			LeaderboardID: "weekly",
			PublicID:      "player-2",
			Score:         110,
			Rank:          3,
		}}})
		requireProtoEqual(t, service.getAroundMemberRequest, &api.GetAroundMemberRequest{
			LeaderboardId:     "weekly",
			MemberPublicId:    "player-1",
			Order:             "asc",
			GetLastIfNotFound: true,
			PageSize:          5,
		})
	})

	t.Run("get top", func(t *testing.T) {
		got, err := client.GetTop(context.Background(), "weekly", 2, 10)
		requireNoError(t, err)
		requireEqual(t, got, &MemberList{Members: []*Member{{
			LeaderboardID: "weekly",
			PublicID:      "player-1",
			Score:         120,
			Rank:          2,
		}}})
		requireProtoEqual(t, service.getTopMembersRequest, &api.GetTopMembersRequest{
			LeaderboardId: "weekly",
			PageNumber:    2,
			PageSize:      10,
		})
	})

	t.Run("get top percent", func(t *testing.T) {
		got, err := client.GetTopPercent(context.Background(), "weekly", 10)
		requireNoError(t, err)
		requireEqual(t, got, &MemberList{Members: []*Member{{
			LeaderboardID: "weekly",
			PublicID:      "player-1",
			Score:         120,
			Rank:          2,
		}}})
		requireProtoEqual(t, service.getTopPercentageRequest, &api.GetTopPercentageRequest{
			LeaderboardId: "weekly",
			Percentage:    10,
		})
	})

	t.Run("increment score", func(t *testing.T) {
		got, err := client.IncrementScore(context.Background(), "weekly", "player-1", 5, 60)
		requireNoError(t, err)
		requireEqual(t, got, &Member{
			LeaderboardID: "weekly",
			PublicID:      "player-1",
			Score:         125,
			Rank:          1,
			PreviousRank:  2,
		})
		requireProtoEqual(t, service.incrementScoreRequest, &api.IncrementScoreRequest{
			LeaderboardId:  "weekly",
			MemberPublicId: "player-1",
			ScoreTtl:       60,
			Body: &api.IncrementScoreRequest_Body{
				Increment: 5,
			},
		})
	})

	t.Run("remove member", func(t *testing.T) {
		got, err := client.RemoveMemberFromLeaderboard(context.Background(), "weekly", "player-1")
		requireNoError(t, err)
		requireEqual(t, got, &Response{Success: true, Reason: "removed"})
		requireProtoEqual(t, service.removeMemberRequest, &api.RemoveMemberRequest{
			LeaderboardId:  "weekly",
			MemberPublicId: "player-1",
		})
	})

	t.Run("update score", func(t *testing.T) {
		got, err := client.UpdateScore(context.Background(), "weekly", "player-1", 120, 60)
		requireNoError(t, err)
		requireEqual(t, got, &Member{
			LeaderboardID: "weekly",
			PublicID:      "player-1",
			Score:         120,
			Rank:          2,
			PreviousRank:  3,
		})
		requireProtoEqual(t, service.upsertScoreRequest, &api.UpsertScoreRequest{
			LeaderboardId:  "weekly",
			MemberPublicId: "player-1",
			PrevRank:       true,
			ScoreTtl:       60,
			ScoreChange: &api.UpsertScoreRequest_ScoreChange{
				Score: 120,
			},
		})
	})

	t.Run("update scores", func(t *testing.T) {
		got, err := client.UpdateScores(
			context.Background(),
			[]string{"weekly", "monthly"},
			"player-1",
			120,
			60,
		)
		requireNoError(t, err)
		requireEqual(t, got, &ScoreList{Scores: []*Score{{
			LeaderboardID: "weekly",
			PublicID:      "player-1",
			Score:         120,
			Rank:          2,
			PreviousRank:  3,
		}}})
		requireProtoEqual(t, service.upsertScoreMultiLeaderboardsRequest, &api.UpsertScoreMultiLeaderboardsRequest{
			MemberPublicId: "player-1",
			ScoreTtl:       60,
			PrevRank:       true,
			ScoreMultiChange: &api.UpsertScoreMultiLeaderboardsRequest_ScoreMultiChange{
				Score:        120,
				Leaderboards: []string{"weekly", "monthly"},
			},
		})
	})

	t.Run("update members score", func(t *testing.T) {
		got, err := client.UpdateMembersScore(context.Background(), "weekly", []*Member{{
			PublicID: "player-1",
			Score:    120,
		}}, 60)
		requireNoError(t, err)
		requireEqual(t, got, &MemberList{Members: []*Member{{
			LeaderboardID: "weekly",
			PublicID:      "player-1",
			Score:         120,
			Rank:          2,
			PreviousRank:  3,
		}}})
		requireProtoEqual(t, service.bulkUpsertScoresRequest, &api.BulkUpsertScoresRequest{
			LeaderboardId: "weekly",
			PrevRank:      true,
			ScoreTtl:      60,
			MemberScores: &api.BulkUpsertScoresRequest_MemberScores{
				Members: []*api.BulkUpsertScoresRequest_MemberScore{{
					PublicId: "player-1",
					Score:    120,
				}},
			},
		})
	})
}

type recordingPodiumServer struct {
	api.UnimplementedPodiumServiceServer

	removeLeaderboardRequest            *api.RemoveLeaderboardRequest
	bulkUpsertScoresRequest             *api.BulkUpsertScoresRequest
	upsertScoreRequest                  *api.UpsertScoreRequest
	totalMembersRequest                 *api.TotalMembersRequest
	incrementScoreRequest               *api.IncrementScoreRequest
	getMemberRequest                    *api.GetMemberRequest
	getMembersRequest                   *api.GetMembersRequest
	removeMemberRequest                 *api.RemoveMemberRequest
	getAroundMemberRequest              *api.GetAroundMemberRequest
	getTopMembersRequest                *api.GetTopMembersRequest
	getTopPercentageRequest             *api.GetTopPercentageRequest
	upsertScoreMultiLeaderboardsRequest *api.UpsertScoreMultiLeaderboardsRequest
	getRankMultiLeaderboardsRequest     *api.GetRankMultiLeaderboardsRequest
}

func (*recordingPodiumServer) HealthCheck(
	context.Context,
	*api.HealthCheckRequest,
) (*api.HealthCheckResponse, error) {
	return &api.HealthCheckResponse{WorkingString: "WORKING"}, nil
}

func (s *recordingPodiumServer) RemoveLeaderboard(
	_ context.Context,
	req *api.RemoveLeaderboardRequest,
) (*api.RemoveLeaderboardResponse, error) {
	s.removeLeaderboardRequest = req
	return &api.RemoveLeaderboardResponse{Success: true, Reason: "deleted"}, nil
}

func (s *recordingPodiumServer) BulkUpsertScores(
	_ context.Context,
	req *api.BulkUpsertScoresRequest,
) (*api.BulkUpsertScoresResponse, error) {
	s.bulkUpsertScoresRequest = req
	return &api.BulkUpsertScoresResponse{Success: true, Members: []*api.BulkUpsertScoresResponse_Member{{
		PublicId:     "player-1",
		Score:        120,
		Rank:         2,
		PreviousRank: 3,
	}}}, nil
}

func (s *recordingPodiumServer) UpsertScore(
	_ context.Context,
	req *api.UpsertScoreRequest,
) (*api.UpsertScoreResponse, error) {
	s.upsertScoreRequest = req
	return &api.UpsertScoreResponse{
		Success:      true,
		PublicId:     "player-1",
		Score:        120,
		Rank:         2,
		PreviousRank: 3,
	}, nil
}

func (s *recordingPodiumServer) TotalMembers(
	_ context.Context,
	req *api.TotalMembersRequest,
) (*api.TotalMembersResponse, error) {
	s.totalMembersRequest = req
	return &api.TotalMembersResponse{Success: true, Count: 42}, nil
}

func (s *recordingPodiumServer) IncrementScore(
	_ context.Context,
	req *api.IncrementScoreRequest,
) (*api.IncrementScoreResponse, error) {
	s.incrementScoreRequest = req
	return &api.IncrementScoreResponse{
		Success:      true,
		PublicId:     "player-1",
		Score:        125,
		Rank:         1,
		PreviousRank: 2,
	}, nil
}

func (s *recordingPodiumServer) GetMember(
	_ context.Context,
	req *api.GetMemberRequest,
) (*api.GetMemberResponse, error) {
	s.getMemberRequest = req
	return &api.GetMemberResponse{
		Success:      true,
		PublicId:     "player-1",
		Score:        120,
		Rank:         2,
		PreviousRank: 3,
	}, nil
}

func (s *recordingPodiumServer) GetMembers(
	_ context.Context,
	req *api.GetMembersRequest,
) (*api.GetMembersResponse, error) {
	s.getMembersRequest = req
	return &api.GetMembersResponse{
		Success: true,
		Members: []*api.GetMembersResponse_Member{{
			PublicId: "player-1",
			Score:    120,
			Rank:     2,
		}},
		NotFound: []string{"missing"},
	}, nil
}

func (s *recordingPodiumServer) RemoveMember(
	_ context.Context,
	req *api.RemoveMemberRequest,
) (*api.RemoveMemberResponse, error) {
	s.removeMemberRequest = req
	return &api.RemoveMemberResponse{Success: true, Reason: "removed"}, nil
}

func (s *recordingPodiumServer) GetAroundMember(
	_ context.Context,
	req *api.GetAroundMemberRequest,
) (*api.GetAroundMemberResponse, error) {
	s.getAroundMemberRequest = req
	return &api.GetAroundMemberResponse{Success: true, Members: []*api.Member{{
		PublicId: "player-2",
		Score:    110,
		Rank:     3,
	}}}, nil
}

func (s *recordingPodiumServer) GetTopMembers(
	_ context.Context,
	req *api.GetTopMembersRequest,
) (*api.GetTopMembersResponse, error) {
	s.getTopMembersRequest = req
	return &api.GetTopMembersResponse{Success: true, Members: []*api.Member{{
		PublicId: "player-1",
		Score:    120,
		Rank:     2,
	}}}, nil
}

func (s *recordingPodiumServer) GetTopPercentage(
	_ context.Context,
	req *api.GetTopPercentageRequest,
) (*api.GetTopPercentageResponse, error) {
	s.getTopPercentageRequest = req
	return &api.GetTopPercentageResponse{Success: true, Members: []*api.Member{{
		PublicId: "player-1",
		Score:    120,
		Rank:     2,
	}}}, nil
}

func (s *recordingPodiumServer) UpsertScoreMultiLeaderboards(
	_ context.Context,
	req *api.UpsertScoreMultiLeaderboardsRequest,
) (*api.UpsertScoreMultiLeaderboardsResponse, error) {
	s.upsertScoreMultiLeaderboardsRequest = req
	return &api.UpsertScoreMultiLeaderboardsResponse{
		Success: true,
		Scores: []*api.UpsertScoreMultiLeaderboardsResponse_Member{{
			LeaderboardId: "weekly",
			PublicId:      "player-1",
			Score:         120,
			Rank:          2,
			PreviousRank:  3,
		}},
	}, nil
}

func (s *recordingPodiumServer) GetRankMultiLeaderboards(
	_ context.Context,
	req *api.GetRankMultiLeaderboardsRequest,
) (*api.GetRankMultiLeaderboardsResponse, error) {
	s.getRankMultiLeaderboardsRequest = req
	return &api.GetRankMultiLeaderboardsResponse{
		Success: true,
		Scores: []*api.GetRankMultiLeaderboardsResponse_Member{{
			LeaderboardId: "weekly",
			Score:         120,
			Rank:          2,
		}},
	}, nil
}

func newTestGRPCClient(t *testing.T, service api.PodiumServiceServer) *GRPCClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	api.RegisterPodiumServiceServer(server, service)
	go func() {
		_ = server.Serve(listener)
	}()

	client, err := NewGRPC(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	requireNoError(t, err)

	t.Cleanup(func() {
		requireNoError(t, client.Close())
		server.Stop()
		requireNoError(t, listener.Close())
	})

	return client
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func requireEqual(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func requireProtoEqual(t *testing.T, got, want proto.Message) {
	t.Helper()
	if !proto.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
