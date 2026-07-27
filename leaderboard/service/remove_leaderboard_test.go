package service_test

import (
	"context"

	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/service"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Service RemoveLeaderboard", func() {
	var ctrl *gomock.Controller
	var mock *database.MockDatabase
	var svc *service.Service

	var leaderboard string = "testKey"

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mock = database.NewMockDatabase(ctrl)

		svc = &service.Service{mock}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("Should return nil if all is OK", func() {
		mock.EXPECT().RemoveLeaderboard(gomock.Any(), gomock.Eq(leaderboard)).Return(nil)

		err := svc.RemoveLeaderboard(context.Background(), leaderboard)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should return error if database return in error", func() {
		mock.EXPECT().RemoveLeaderboard(gomock.Any(), gomock.Eq(leaderboard)).Return(database.NewGeneralError("unknown error"))

		err := svc.RemoveLeaderboard(context.Background(), leaderboard)
		Expect(err).To(
			Equal(
				service.NewGeneralError(
					"remove leaderboard",
					database.NewGeneralError("unknown error").Error(),
				),
			),
		)
	})
})
