package service_test

import (
	"context"
	"fmt"

	"github.com/TeneficGames/podium/leaderboard/v2/database"
	"github.com/TeneficGames/podium/leaderboard/v2/service"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Service RemoveMembers", func() {
	var ctrl *gomock.Controller
	var mock *database.MockDatabase
	var svc *service.Service

	var leaderboard string = "leaderboardTest"
	var members []string = []string{"member", "member2"}

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mock = database.NewMockDatabase(ctrl)

		svc = &service.Service{mock}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("Should return nil if all is OK", func() {
		mock.EXPECT().RemoveMembers(gomock.Any(), gomock.Eq(leaderboard), gomock.Eq(members)).Return(nil)

		err := svc.RemoveMembers(context.Background(), leaderboard, members)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should return error if database return in error", func() {
		mock.EXPECT().RemoveMembers(gomock.Any(), gomock.Eq(leaderboard), gomock.Eq(members)).Return(fmt.Errorf("unknown error"))

		err := svc.RemoveMembers(context.Background(), leaderboard, members)
		Expect(err).To(Equal(service.NewGeneralError("remove members", "unknown error")))
	})
})
