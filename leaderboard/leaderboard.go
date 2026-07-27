package leaderboard

import (
	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/service"
)

var _ service.Leaderboard = &service.Service{}
var _ database.Database = &database.Redis{}
var _ database.Expiration = &database.Redis{}
