// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games
package cmd

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/spf13/cobra"
)

func Test(t *testing.T) {
	Describe("Root Cmd", func() {
		It("Should run command", func() {
			var rootCmd = &cobra.Command{
				Use:   "podium",
				Short: "podium handles redis backed leaderboards",
				Long:  `podium handles redis backed leaderboards.`,
			}
			Execute(rootCmd)
		})
	})
}
