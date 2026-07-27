package service_test

import (
	"context"
	"fmt"

	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/model"
	"github.com/TeneficGames/podium/leaderboard/service"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

type fastPathDatabase struct {
	database.Database
	ranks        []int64
	member       *database.Member
	writeErr     error
	incrementErr error
}

func (d *fastPathDatabase) SetMembersAndGetRanks(
	context.Context,
	string,
	string,
	[]*database.Member,
) ([]int64, error) {
	return d.ranks, d.writeErr
}

func (d *fastPathDatabase) IncrementMemberScoreAndGetRank(
	context.Context,
	string,
	string,
	string,
	float64,
) (*database.Member, error) {
	return d.member, d.incrementErr
}

var _ = Describe("Service pipelined fast paths", func() {
	var (
		ctrl *gomock.Controller
		mock *database.MockDatabase
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mock = database.NewMockDatabase(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("sets member scores and resulting ranks in one database operation", func() {
		fastDatabase := &fastPathDatabase{
			Database: mock,
			ranks:    []int64{1, 0},
		}
		svc := service.NewService(fastDatabase)
		members := []*model.Member{
			{PublicID: "member-1", Score: 10},
			{PublicID: "member-2", Score: 20},
		}

		err := svc.SetMembersScore(context.Background(), "leaderboard", members, false, "")

		Expect(err).NotTo(HaveOccurred())
		Expect(members[0].Rank).To(Equal(2))
		Expect(members[1].Rank).To(Equal(1))
	})

	It("returns a pipelined member write error", func() {
		fastDatabase := &fastPathDatabase{
			Database: mock,
			writeErr: fmt.Errorf("write error"),
		}
		svc := service.NewService(fastDatabase)

		err := svc.SetMembersScore(
			context.Background(),
			"leaderboard",
			[]*model.Member{{PublicID: "member-1", Score: 10}},
			false,
			"",
		)

		Expect(err).To(MatchError(ContainSubstring("write error")))
	})

	It("increments a member and returns its score and rank in one database operation", func() {
		fastDatabase := &fastPathDatabase{
			Database: mock,
			member: &database.Member{
				Member: "member-1",
				Score:  12,
				Rank:   2,
			},
		}
		svc := service.NewService(fastDatabase)

		member, err := svc.IncrementMemberScore(
			context.Background(),
			"leaderboard",
			"member-1",
			2,
			"",
		)

		Expect(err).NotTo(HaveOccurred())
		Expect(member).To(Equal(&model.Member{
			PublicID: "member-1",
			Score:    12,
			Rank:     3,
		}))
	})

	It("returns a pipelined increment error", func() {
		fastDatabase := &fastPathDatabase{
			Database:     mock,
			incrementErr: fmt.Errorf("increment error"),
		}
		svc := service.NewService(fastDatabase)

		_, err := svc.IncrementMemberScore(
			context.Background(),
			"leaderboard",
			"member-1",
			2,
			"",
		)

		Expect(err).To(MatchError(ContainSubstring("increment error")))
	})
})
