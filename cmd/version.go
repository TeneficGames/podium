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
	"fmt"

	"github.com/TeneficGames/podium/api"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "returns Podium version",
	Long:  `returns Podium version`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Podium v%s\n", api.VERSION)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
