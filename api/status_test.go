// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package api_test

import (
	"context"
	"encoding/json"
	"net/http"

	empty "google.golang.org/protobuf/types/known/emptypb"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	api "github.com/TeneficGames/podium/proto/podium/api/v1"
	"github.com/TeneficGames/podium/testing"
)

var _ = Describe("Status Handler", func() {
	It("Should respond with status (http)", func() {
		a := testing.GetDefaultTestApp()
		status, body := testing.Get(a, "/status")

		Expect(status).To(Equal(http.StatusOK))

		var result map[string]interface{}
		json.Unmarshal([]byte(body), &result)

		Expect(result["app"]).NotTo(BeNil())

		app := result["app"].(map[string]interface{})
		Expect(app["errorRate"]).To(BeEquivalentTo(0.0))
	})

	It("Should respond with status (grpc)", func() {
		a := testing.GetDefaultTestApp()

		testing.SetupGRPC(a, func(cli api.PodiumClient) {
			resp, err := cli.Status(context.Background(), &empty.Empty{})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.ErrorRate).To(BeEquivalentTo(0.0))
		})
	})
})
