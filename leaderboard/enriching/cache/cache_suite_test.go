package cache

import (
	"testing"

	podiumtesting "github.com/TeneficGames/podium/leaderboard/testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	goredis "github.com/redis/go-redis/v9"
)

var testRedisServer *podiumtesting.RedisServer
var testRedisClient *goredis.Client

var _ = BeforeSuite(func() {
	var err error
	testRedisServer, err = podiumtesting.StartRedis()
	Expect(err).NotTo(HaveOccurred())
	testRedisClient = goredis.NewClient(&goredis.Options{Addr: testRedisServer.Addr()})
})

var _ = AfterSuite(func() {
	Expect(testRedisClient.Close()).To(Succeed())
	testRedisServer.Close()
})

func TestEnrichingCache(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Enriching Cache Suite")
}
