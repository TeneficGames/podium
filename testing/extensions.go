// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package testing

import (
	"context"

	"github.com/onsi/ginkgo/v2"
)

// BeforeOnce runs the before each block only once
func BeforeOnce(beforeBlock func()) {
	hasRun := false

	ginkgo.BeforeEach(func() {
		if !hasRun {
			beforeBlock()
			hasRun = true
		}
	})
}

// NewEmptyCtx Creates an empty context (shortcut for context.Background())
func NewEmptyCtx() context.Context {
	return context.Background()
}
