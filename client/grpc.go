package client

import (
	"context"
	"strings"

	api "github.com/TeneficGames/podium/proto/podium/api/v1"
	"google.golang.org/grpc"
)

// GRPCClient is a Podium client that communicates over gRPC.
type GRPCClient struct {
	conn   *grpc.ClientConn
	client api.PodiumServiceClient
}

var _ Client = (*GRPCClient)(nil)

// NewGRPC creates a Podium gRPC client. Dial options configure transport
// credentials, authentication, interceptors, and other connection behavior.
func NewGRPC(target string, opts ...grpc.DialOption) (*GRPCClient, error) {
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}

	return &GRPCClient{
		conn:   conn,
		client: api.NewPodiumServiceClient(conn),
	}, nil
}

// Close closes the underlying gRPC connection.
func (p *GRPCClient) Close() error {
	return p.conn.Close()
}

// DeleteLeaderboard deletes the leaderboard from Podium.
func (p *GRPCClient) DeleteLeaderboard(ctx context.Context, leaderboard string) (*Response, error) {
	res, err := p.client.RemoveLeaderboard(requestContext(ctx), &api.RemoveLeaderboardRequest{
		LeaderboardId: leaderboard,
	})
	if err != nil {
		return nil, err
	}

	return &Response{Success: res.GetSuccess(), Reason: res.GetReason()}, nil
}

// GetCount gets the number of members in a leaderboard.
func (p *GRPCClient) GetCount(ctx context.Context, leaderboard string) (int, error) {
	res, err := p.client.TotalMembers(requestContext(ctx), &api.TotalMembersRequest{
		LeaderboardId: leaderboard,
	})
	if err != nil {
		return 0, err
	}

	return int(res.GetCount()), nil
}

// GetMember shows the score and rank of a member in a leaderboard.
func (p *GRPCClient) GetMember(ctx context.Context, leaderboard, memberID string) (*Member, error) {
	res, err := p.client.GetMember(requestContext(ctx), &api.GetMemberRequest{
		LeaderboardId:  leaderboard,
		MemberPublicId: memberID,
	})
	if err != nil {
		return nil, err
	}

	return &Member{
		LeaderboardID: leaderboard,
		PublicID:      res.GetPublicId(),
		Score:         int(res.GetScore()),
		Rank:          int(res.GetRank()),
		PreviousRank:  int(res.GetPreviousRank()),
	}, nil
}

// GetMembers returns members from a leaderboard.
func (p *GRPCClient) GetMembers(ctx context.Context, leaderboard string, memberIDs []string) (*MemberList, error) {
	res, err := p.client.GetMembers(requestContext(ctx), &api.GetMembersRequest{
		LeaderboardId: leaderboard,
		Ids:           strings.Join(memberIDs, ","),
	})
	if err != nil {
		return nil, err
	}

	members := make([]*Member, len(res.GetMembers()))
	for i, member := range res.GetMembers() {
		members[i] = &Member{
			LeaderboardID: leaderboard,
			PublicID:      member.GetPublicId(),
			Score:         int(member.GetScore()),
			Rank:          int(member.GetRank()),
		}
	}

	return &MemberList{Members: members, NotFound: res.GetNotFound()}, nil
}

// GetMemberInLeaderboards returns a member's rank and score in multiple leaderboards.
func (p *GRPCClient) GetMemberInLeaderboards(
	ctx context.Context,
	leaderboards []string,
	memberID string,
	order ...string,
) (*ScoreList, error) {
	res, err := p.client.GetRankMultiLeaderboards(requestContext(ctx), &api.GetRankMultiLeaderboardsRequest{
		MemberPublicId: memberID,
		LeaderboardIds: strings.Join(leaderboards, ","),
		Order:          requestedOrder(order),
	})
	if err != nil {
		return nil, err
	}

	scores := make([]*Score, len(res.GetScores()))
	for i, score := range res.GetScores() {
		scores[i] = &Score{
			LeaderboardID: score.GetLeaderboardId(),
			PublicID:      memberID,
			Score:         int(score.GetScore()),
			Rank:          int(score.GetRank()),
		}
	}

	return &ScoreList{Scores: scores}, nil
}

// GetMembersAroundMember returns members around a member in a leaderboard.
func (p *GRPCClient) GetMembersAroundMember(
	ctx context.Context,
	leaderboard, memberID string,
	pageSize int,
	getLastIfNotFound bool,
	order ...string,
) (*MemberList, error) {
	res, err := p.client.GetAroundMember(requestContext(ctx), &api.GetAroundMemberRequest{
		LeaderboardId:     leaderboard,
		MemberPublicId:    memberID,
		Order:             requestedOrder(order),
		GetLastIfNotFound: getLastIfNotFound,
		PageSize:          int32(pageSize),
	})
	if err != nil {
		return nil, err
	}

	return &MemberList{Members: membersFromProto(leaderboard, res.GetMembers())}, nil
}

// GetTop returns the top members for a leaderboard. Page is 1-indexed.
func (p *GRPCClient) GetTop(ctx context.Context, leaderboard string, page, pageSize int) (*MemberList, error) {
	res, err := p.client.GetTopMembers(requestContext(ctx), &api.GetTopMembersRequest{
		LeaderboardId: leaderboard,
		PageNumber:    int32(page),
		PageSize:      int32(pageSize),
	})
	if err != nil {
		return nil, err
	}

	return &MemberList{Members: membersFromProto(leaderboard, res.GetMembers())}, nil
}

// GetTopPercent returns the top percentage of members in a leaderboard.
func (p *GRPCClient) GetTopPercent(ctx context.Context, leaderboard string, percentage int) (*MemberList, error) {
	res, err := p.client.GetTopPercentage(requestContext(ctx), &api.GetTopPercentageRequest{
		LeaderboardId: leaderboard,
		Percentage:    int32(percentage),
	})
	if err != nil {
		return nil, err
	}

	return &MemberList{Members: membersFromProto(leaderboard, res.GetMembers())}, nil
}

// Healthcheck verifies whether Podium is available.
func (p *GRPCClient) Healthcheck(ctx context.Context) (string, error) {
	res, err := p.client.HealthCheck(requestContext(ctx), &api.HealthCheckRequest{})
	if err != nil {
		return "", err
	}

	return res.GetWorkingString(), nil
}

// IncrementScore increments a member's score in a leaderboard.
func (p *GRPCClient) IncrementScore(
	ctx context.Context,
	leaderboard, memberID string,
	increment, scoreTTL int,
) (*Member, error) {
	res, err := p.client.IncrementScore(requestContext(ctx), &api.IncrementScoreRequest{
		LeaderboardId:  leaderboard,
		MemberPublicId: memberID,
		ScoreTtl:       int32(scoreTTL),
		Body: &api.IncrementScoreRequest_Body{
			Increment: float64(increment),
		},
	})
	if err != nil {
		return nil, err
	}

	return &Member{
		LeaderboardID: leaderboard,
		PublicID:      res.GetPublicId(),
		Score:         int(res.GetScore()),
		Rank:          int(res.GetRank()),
		PreviousRank:  int(res.GetPreviousRank()),
	}, nil
}

// RemoveMemberFromLeaderboard removes a member from a leaderboard.
func (p *GRPCClient) RemoveMemberFromLeaderboard(
	ctx context.Context,
	leaderboard, memberID string,
) (*Response, error) {
	res, err := p.client.RemoveMember(requestContext(ctx), &api.RemoveMemberRequest{
		LeaderboardId:  leaderboard,
		MemberPublicId: memberID,
	})
	if err != nil {
		return nil, err
	}

	return &Response{Success: res.GetSuccess(), Reason: res.GetReason()}, nil
}

// UpdateScore updates a member's score in a leaderboard.
func (p *GRPCClient) UpdateScore(
	ctx context.Context,
	leaderboard, memberID string,
	score, scoreTTL int,
) (*Member, error) {
	res, err := p.client.UpsertScore(requestContext(ctx), &api.UpsertScoreRequest{
		LeaderboardId:  leaderboard,
		MemberPublicId: memberID,
		PrevRank:       true,
		ScoreTtl:       int32(scoreTTL),
		ScoreChange: &api.UpsertScoreRequest_ScoreChange{
			Score: float64(score),
		},
	})
	if err != nil {
		return nil, err
	}

	return &Member{
		LeaderboardID: leaderboard,
		PublicID:      res.GetPublicId(),
		Score:         int(res.GetScore()),
		Rank:          int(res.GetRank()),
		PreviousRank:  int(res.GetPreviousRank()),
	}, nil
}

// UpdateScores updates a member's score in multiple leaderboards.
func (p *GRPCClient) UpdateScores(
	ctx context.Context,
	leaderboards []string,
	memberID string,
	score, scoreTTL int,
) (*ScoreList, error) {
	res, err := p.client.UpsertScoreMultiLeaderboards(
		requestContext(ctx),
		&api.UpsertScoreMultiLeaderboardsRequest{
			MemberPublicId: memberID,
			ScoreTtl:       int32(scoreTTL),
			PrevRank:       true,
			ScoreMultiChange: &api.UpsertScoreMultiLeaderboardsRequest_ScoreMultiChange{
				Score:        float64(score),
				Leaderboards: leaderboards,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	scores := make([]*Score, len(res.GetScores()))
	for i, result := range res.GetScores() {
		scores[i] = &Score{
			LeaderboardID: result.GetLeaderboardId(),
			PublicID:      result.GetPublicId(),
			Score:         int(result.GetScore()),
			Rank:          int(result.GetRank()),
			PreviousRank:  int(result.GetPreviousRank()),
		}
	}

	return &ScoreList{Scores: scores}, nil
}

// UpdateMembersScore updates multiple members' scores in a leaderboard.
func (p *GRPCClient) UpdateMembersScore(
	ctx context.Context,
	leaderboard string,
	members []*Member,
	scoreTTL int,
) (*MemberList, error) {
	memberScores := make([]*api.BulkUpsertScoresRequest_MemberScore, len(members))
	for i, member := range members {
		memberScores[i] = &api.BulkUpsertScoresRequest_MemberScore{
			PublicId: member.PublicID,
			Score:    float64(member.Score),
		}
	}

	res, err := p.client.BulkUpsertScores(requestContext(ctx), &api.BulkUpsertScoresRequest{
		LeaderboardId: leaderboard,
		PrevRank:      true,
		ScoreTtl:      int32(scoreTTL),
		MemberScores: &api.BulkUpsertScoresRequest_MemberScores{
			Members: memberScores,
		},
	})
	if err != nil {
		return nil, err
	}

	result := make([]*Member, len(res.GetMembers()))
	for i, member := range res.GetMembers() {
		result[i] = &Member{
			LeaderboardID: leaderboard,
			PublicID:      member.GetPublicId(),
			Score:         int(member.GetScore()),
			Rank:          int(member.GetRank()),
			PreviousRank:  int(member.GetPreviousRank()),
		}
	}

	return &MemberList{Members: result}, nil
}

func membersFromProto(leaderboard string, members []*api.Member) []*Member {
	result := make([]*Member, len(members))
	for i, member := range members {
		result[i] = &Member{
			LeaderboardID: leaderboard,
			PublicID:      member.GetPublicId(),
			Score:         int(member.GetScore()),
			Rank:          int(member.GetRank()),
		}
	}
	return result
}

func requestContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func requestedOrder(order []string) string {
	if len(order) > 0 {
		return order[0]
	}
	return "desc"
}
