// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package enriching

import (
	"context"
	"testing"
	"time"

	"github.com/TeneficGames/podium/leaderboard/model"
	"go.uber.org/zap"
)

func TestEnricherOptions(t *testing.T) {
	const timeout = 2 * time.Second
	impl := &enricherImpl{config: newDefaultEnrichConfig()}

	WithWebhookTimeout(timeout)(impl)
	WithLogger(zap.NewNop())(impl)

	if impl.config.webhookTimeout != timeout {
		t.Fatalf("expected webhook timeout %s, got %s", timeout, impl.config.webhookTimeout)
	}
	if impl.logger == nil {
		t.Fatal("expected logger to be configured")
	}
}

func TestEnrichReturnsEmptyMembersWithoutCallingAProvider(t *testing.T) {
	enricher := NewEnricher()
	members := []*model.Member{}

	result, err := enricher.Enrich(context.Background(), "tenant", "leaderboard", members)
	if err != nil {
		t.Fatalf("enrich empty members: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected no members, got %d", len(result))
	}
}
