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
	"testing"
	"time"

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
