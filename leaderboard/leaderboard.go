package leaderboard

import (
	"github.com/TeneficGames/podium/leaderboard/v2/database"
	"github.com/TeneficGames/podium/leaderboard/v2/service"
)

var _ service.Leaderboard = &service.Service{}
var _ database.Database = &database.Redis{}
var _ database.Expiration = &database.Redis{}
