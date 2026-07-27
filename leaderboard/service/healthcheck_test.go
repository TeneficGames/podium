package service_test

import (
	"context"
	"fmt"

	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/service"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Service Healthcheck", func() {
	var ctrl *gomock.Controller
	var mock *database.MockDatabase
	var svc *service.Service

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mock = database.NewMockDatabase(ctrl)

		svc = &service.Service{mock}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("Should return nil if database returns no error", func() {
		mock.EXPECT().Healthcheck(gomock.Any()).Return(nil)

		err := svc.Healthcheck(context.Background())
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should return error if database return in error", func() {
		mock.EXPECT().Healthcheck(gomock.Any()).Return(fmt.Errorf("Database error example"))

		err := svc.Healthcheck(context.Background())
		Expect(err).To(Equal(service.NewGeneralError("healthcheck", "Database error example")))
	})
})
