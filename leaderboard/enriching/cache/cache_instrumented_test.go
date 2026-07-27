package cache

import (
	"context"
	"errors"

	mock_enriching "github.com/TeneficGames/podium/leaderboard/mocks"
	"github.com/TeneficGames/podium/leaderboard/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Instrumented enrich cache Get tests", func() {
	tenantID := "tenant-id"
	members := []*model.Member{
		{
			PublicID: "member1",
		},
	}

	result := map[string]map[string]string{
		"member1": {
			"field1": "value1",
		},
	}

	It("should send metrics when Get is called successfully with hit", func() {
		ctrl := gomock.NewController(GinkgoT())
		impl := mock_enriching.NewMockEnricherCache(ctrl)

		impl.EXPECT().Get(gomock.Any(), tenantID, members).Return(result, true, nil)

		instrumentedCache, err := NewInstrumentedCache(impl)
		Expect(err).NotTo(HaveOccurred())
		res, hit, err := instrumentedCache.Get(context.Background(), tenantID, members)

		Expect(res).To(Equal(result))
		Expect(hit).To(BeTrue())
		Expect(err).To(BeNil())
	})

	It("should send metrics when Get is called successfully with miss", func() {
		ctrl := gomock.NewController(GinkgoT())
		impl := mock_enriching.NewMockEnricherCache(ctrl)

		impl.EXPECT().Get(gomock.Any(), tenantID, members).Return(nil, false, nil)

		instrumentedCache, err := NewInstrumentedCache(impl)
		Expect(err).NotTo(HaveOccurred())
		res, hit, err := instrumentedCache.Get(context.Background(), tenantID, members)
		Expect(res).To(BeNil())
		Expect(hit).To(BeFalse())
		Expect(err).To(BeNil())
	})

	It("should send metrics when Get is called with error", func() {
		ctrl := gomock.NewController(GinkgoT())
		impl := mock_enriching.NewMockEnricherCache(ctrl)

		impl.EXPECT().Get(gomock.Any(), tenantID, members).Return(nil, false, errors.New("error"))

		instrumentedCache, err := NewInstrumentedCache(impl)
		Expect(err).NotTo(HaveOccurred())
		res, hit, err := instrumentedCache.Get(context.Background(), tenantID, members)

		Expect(res).To(BeNil())
		Expect(hit).To(BeFalse())
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Instrumented enrich cache Set tests", func() {
	tenantID := "tenant-id"
	members := []*model.Member{
		{
			PublicID: "member1",
			Metadata: map[string]string{
				"field1": "value1",
			},
		},
	}

	It("should send metrics when Set is called successfully", func() {
		ctrl := gomock.NewController(GinkgoT())
		impl := mock_enriching.NewMockEnricherCache(ctrl)

		impl.EXPECT().Set(gomock.Any(), tenantID, members, gomock.Any()).Return(nil)

		instrumentedCache, err := NewInstrumentedCache(impl)
		Expect(err).NotTo(HaveOccurred())
		err = instrumentedCache.Set(context.Background(), tenantID, members, 0)

		Expect(err).To(BeNil())
	})

	It("should send metrics when Set is called with error", func() {
		ctrl := gomock.NewController(GinkgoT())
		impl := mock_enriching.NewMockEnricherCache(ctrl)

		impl.EXPECT().Set(gomock.Any(), tenantID, members, gomock.Any()).Return(errors.New("error"))

		instrumentedCache, err := NewInstrumentedCache(impl)
		Expect(err).NotTo(HaveOccurred())
		err = instrumentedCache.Set(context.Background(), tenantID, members, 0)

		Expect(err).To(HaveOccurred())
	})
})
