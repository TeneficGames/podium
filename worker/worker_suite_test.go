package worker_test

import (
	"fmt"
	"os"
	"testing"

	podiumtesting "github.com/TeneficGames/podium/leaderboard/testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var testRedisServer *podiumtesting.RedisServer

var _ = BeforeSuite(func() {
	var err error
	testRedisServer, err = podiumtesting.StartRedis()
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Setenv("PODIUM_REDIS_CLUSTER_ENABLED", "false")).To(Succeed())
	Expect(os.Setenv("PODIUM_REDIS_HOST", testRedisServer.Host)).To(Succeed())
	Expect(os.Setenv("PODIUM_REDIS_PORT", fmt.Sprint(testRedisServer.Port))).To(Succeed())
	Expect(os.Setenv("PODIUM_REDIS_DB", "1")).To(Succeed())
})

var _ = AfterSuite(func() {
	testRedisServer.Close()
	Expect(os.Unsetenv("PODIUM_REDIS_CLUSTER_ENABLED")).To(Succeed())
	Expect(os.Unsetenv("PODIUM_REDIS_HOST")).To(Succeed())
	Expect(os.Unsetenv("PODIUM_REDIS_PORT")).To(Succeed())
	Expect(os.Unsetenv("PODIUM_REDIS_DB")).To(Succeed())
})

func TestApi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker Suite")
}
