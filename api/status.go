// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	api "github.com/TeneficGames/podium/proto/podium/api/v1"
)

type statusPayload struct {
	App statusApp `json:"app"`
}

type statusApp struct {
	ErrorRate float64 `json:"errorRate"`
}

// statusHandler is the handler responsible for reporting podium status.
func (app *App) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	payload := statusPayload{App: statusApp{
		ErrorRate: app.Errors.Rate()},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		errMsg := fmt.Sprintf("JSON marshaling failed: %v", err)
		app.Logger.Error(errMsg)
		data = []byte(newFailMsg(errMsg))
		w.WriteHeader(http.StatusInternalServerError)
	}

	if _, err := w.Write(data); err != nil {
		app.Logger.Error("Error writing /status response", zap.Error(err))
	}
}

func (app *App) Status(ctx context.Context, req *api.StatusRequest) (*api.StatusResponse, error) {
	return &api.StatusResponse{ErrorRate: app.Errors.Rate()}, nil
}
