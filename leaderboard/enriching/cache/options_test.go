// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/TeneficGames/podium/leaderboard/model"
	redismock "github.com/go-redis/redismock/v9"
	"go.uber.org/zap"
)

func TestCachedEnricherOptions(t *testing.T) {
	const ttl = 15 * time.Minute
	enricher := &cachedEnricher{config: newDefaultCacheConfig()}

	WithTTL(ttl)(enricher)
	WithLogger(zap.NewNop())(enricher)

	if enricher.config.ttl != ttl {
		t.Fatalf("expected cache TTL %s, got %s", ttl, enricher.config.ttl)
	}
	if enricher.logger == nil {
		t.Fatal("expected logger to be configured")
	}
}

func TestRedisCacheRejectsInvalidJSON(t *testing.T) {
	const tenantID = "tenant"
	redisClient, redisMock := redismock.NewClientMock()
	members := []*model.Member{{PublicID: "member"}}
	redisMock.ExpectMGet(getKeysFromMemberArray(tenantID, members)...).
		SetVal([]interface{}{"invalid"})

	cache := NewEnricherRedisCache(redisClient)
	metadata, hit, err := cache.Get(context.Background(), tenantID, members)
	if err == nil {
		t.Fatal("expected invalid cached JSON to fail")
	}
	if hit || metadata != nil {
		t.Fatalf("expected a cache miss without metadata, got hit=%t metadata=%v", hit, metadata)
	}
}
