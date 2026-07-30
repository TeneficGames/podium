// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package worker_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/TeneficGames/podium/leaderboard/database"
	lservice "github.com/TeneficGames/podium/leaderboard/service"
	"github.com/TeneficGames/podium/worker"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scores Expirer Worker", func() {

	var redisClient *database.Redis
	var expirationWorker *worker.ExpirationWorker
	var leaderboards lservice.Leaderboard

	const lbName string = "test-expire-leaderboard"

	var expirationSink chan []*worker.ExpirationResult
	var errorSink chan error

	runWorker := func(runs int) []*worker.ExpirationResult {
		done := make(chan struct{})
		go func() {
			expirationWorker.Run(expirationSink, errorSink)
			close(done)
		}()

		var results []*worker.ExpirationResult
		for range runs {
			select {
			case results = <-expirationSink:
			case err := <-errorSink:
				expirationWorker.Stop()
				Fail(fmt.Sprintf("expiration worker failed: %v", err))
			case <-time.After(time.Second):
				expirationWorker.Stop()
				Fail("expiration worker did not complete a run")
			}
		}

		expirationWorker.Stop()
		Eventually(done).Should(BeClosed())
		return results
	}

	BeforeEach(func() {
		var err error

		expirationWorker, err = worker.GetExpirationWorker("../config/test.yaml")
		expirationWorker.ExpirationCheckInterval = 10 * time.Millisecond
		expirationSink = make(chan []*worker.ExpirationResult, 10)
		errorSink = make(chan error, 10)
		redisClient = database.NewRedisDatabase(database.RedisOptions{
			ClusterEnabled: expirationWorker.Config.GetBool("redis.cluster.enabled"),
			Addrs:          expirationWorker.Config.GetStringSlice("redis.addrs"),
			Host:           expirationWorker.Config.GetString("redis.host"),
			Port:           expirationWorker.Config.GetInt("redis.port"),
			Password:       expirationWorker.Config.GetString("redis.password"),
			DB:             expirationWorker.Config.GetInt("redis.db"),
		})
		leaderboards = lservice.NewService(redisClient)

		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(leaderboards.RemoveLeaderboard(context.Background(), lbName)).To(Succeed())
		redisClient.Del(context.Background(), database.ExpirationSet)
	})

	It("should expire scores and delete set", func() {
		ttl := "1"
		_, err := leaderboards.SetMemberScore(context.Background(), lbName, "denix", 481516, false, ttl)
		Expect(err).NotTo(HaveOccurred())
		expirationLeaderboards, err := redisClient.GetExpirationLeaderboards(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(expirationLeaderboards).To(ContainElement(lbName))
		member, err := leaderboards.GetMember(context.Background(), lbName, "denix", "desc", true)
		Expect(err).NotTo(HaveOccurred())
		ttlInt, _ := strconv.ParseInt(ttl, 10, 64)
		Expect(member.ExpireAt).To(BeNumerically("~", time.Now().Unix()+ttlInt, 1))
		err = redisClient.SetMembersTTL(context.Background(), lbName, []*database.Member{{
			Member: "denix",
			TTL:    time.Now().Add(-time.Second),
		}})
		Expect(err).NotTo(HaveOccurred())
		runWorker(2)

		total, err := leaderboards.TotalMembers(context.Background(), lbName)
		Expect(err).NotTo(HaveOccurred())
		Expect(total).To(Equal(0))
		expirationLeaderboards, err = redisClient.GetExpirationLeaderboards(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(expirationLeaderboards).To(BeEmpty())
	})

	It("should not expire scores that are in the future", func() {
		ttl := "20"
		_, err := leaderboards.SetMemberScore(context.Background(), lbName, "denix", 481516, false, ttl)
		Expect(err).NotTo(HaveOccurred())
		expirationLeaderboards, err := redisClient.GetExpirationLeaderboards(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(expirationLeaderboards).To(ContainElement(lbName))
		member, err := leaderboards.GetMember(context.Background(), lbName, "denix", "desc", true)
		Expect(err).NotTo(HaveOccurred())
		ttlInt, _ := strconv.ParseInt(ttl, 10, 64)
		Expect(member.ExpireAt).To(BeNumerically("~", time.Now().Unix()+ttlInt, 1))
		runWorker(1)

		total, err := leaderboards.TotalMembers(context.Background(), lbName)
		Expect(err).NotTo(HaveOccurred())
		Expect(total).To(Equal(1))
		expirationLeaderboards, err = redisClient.GetExpirationLeaderboards(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(expirationLeaderboards).To(ConsistOf(lbName))
	})

	It("should not expire scores that are not inserted with scoreTTL set", func() {
		ttl := ""
		_, err := leaderboards.SetMemberScore(context.Background(), lbName, "denix", 481516, false, ttl)
		Expect(err).NotTo(HaveOccurred())
		expirationLeaderboards, err := redisClient.GetExpirationLeaderboards(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(expirationLeaderboards).To(BeEmpty())
		runWorker(1)

		total, err := leaderboards.TotalMembers(context.Background(), lbName)
		Expect(err).NotTo(HaveOccurred())
		Expect(total).To(Equal(1))
	})

	It("a call to expireScores should only remove ExpirationLimitPerRun members from a set", func() {
		expirationWorker.ExpirationLimitPerRun = 1

		ttl := "2"
		_, err := leaderboards.SetMemberScore(context.Background(), lbName, "denix", 481516, false, ttl)
		Expect(err).NotTo(HaveOccurred())
		_, err = leaderboards.SetMemberScore(context.Background(), lbName, "denix2", 481512, false, ttl)
		Expect(err).NotTo(HaveOccurred())
		expirationLeaderboards, err := redisClient.GetExpirationLeaderboards(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(expirationLeaderboards).To(ConsistOf(lbName))
		err = redisClient.SetMembersTTL(context.Background(), lbName, []*database.Member{
			{Member: "denix", TTL: time.Unix(0, 0)},
			{Member: "denix2", TTL: time.Unix(0, 0)},
		})
		Expect(err).NotTo(HaveOccurred())
		runWorker(1)

		total, err := leaderboards.TotalMembers(context.Background(), lbName)
		Expect(err).NotTo(HaveOccurred())
		Expect(total).To(Equal(1))
		_, err = leaderboards.GetMember(context.Background(), lbName, "denix2", "desc", false)
		Expect(err).NotTo(HaveOccurred())
		expirationLeaderboards, err = redisClient.GetExpirationLeaderboards(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(expirationLeaderboards).To(ConsistOf(lbName))
	})

	It("should create a valid expiration worker with external configuration", func() {
		expirationWorker, err := worker.NewExpirationWorker(
			"redis.example",
			6380,
			"secret",
			3,
			2*time.Second,
			42,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(expirationWorker.Config.GetString("redis.host")).To(Equal("redis.example"))
		Expect(expirationWorker.Config.GetInt("redis.port")).To(Equal(6380))
		Expect(expirationWorker.Config.GetString("redis.password")).To(Equal("secret"))
		Expect(expirationWorker.Config.GetInt("redis.db")).To(Equal(3))
		Expect(expirationWorker.ExpirationCheckInterval).To(Equal(2 * time.Second))
		Expect(expirationWorker.ExpirationLimitPerRun).To(Equal(42))
	})

	It("should print correctly expiration results", func() {
		results := []*worker.ExpirationResult{
			{DeletedMembers: 1, DeletedSet: true, Set: "s1"},
			{DeletedMembers: 0, DeletedSet: false, Set: "s2"},
		}

		got := fmt.Sprintf("results: %v", results)
		want :=
			"results: [(DeletedMembers: 1, DeletedSet: true, Set: s1) (DeletedMembers: 0, DeletedSet: false, Set: s2)]"

		Expect(got).To(Equal(want))
	})
})
