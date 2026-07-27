package enriching

import (
	"context"
	"errors"

	mock_enriching "github.com/TeneficGames/podium/leaderboard/mocks"
	"github.com/TeneficGames/podium/leaderboard/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Instrumented enricher", func() {
	var ctrl *gomock.Controller
	ctx := context.Background()
	tenantID := "tenantID"
	leaderboardID := "leaderboardID"
	members := []*model.Member{
		{
			PublicID: "publicID",
		},
	}

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("should return enriched members", func() {
		impl := mock_enriching.NewMockEnricher(ctrl)

		enricher, err := NewInstrumentedEnricher(impl)
		Expect(err).NotTo(HaveOccurred())

		impl.EXPECT().Enrich(gomock.Any(), tenantID, leaderboardID, members).Return(members, nil)

		result, err := enricher.Enrich(ctx, tenantID, leaderboardID, members)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(members))
	})

	It("should return enrichment errors", func() {
		impl := mock_enriching.NewMockEnricher(ctrl)

		enricher, err := NewInstrumentedEnricher(impl)
		Expect(err).NotTo(HaveOccurred())

		impl.EXPECT().Enrich(gomock.Any(), tenantID, leaderboardID, members).Return(nil, errors.New("error"))

		_, err = enricher.Enrich(ctx, tenantID, leaderboardID, members)
		Expect(err).To(MatchError("error"))
	})
})
