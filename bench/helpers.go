// podium
// https://github.com/TeneficGames/podium
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games

package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/TeneficGames/podium/config"
	"github.com/TeneficGames/podium/leaderboard/database"
	"github.com/TeneficGames/podium/leaderboard/service"
	"github.com/onsi/gomega"
)

func getRoute(url string) string {
	return fmt.Sprintf("http://localhost:8888%s", url)
}

func putTo(url string, payload map[string]interface{}) (int, string, error) {
	return sendTo("PUT", url, payload)
}

func patchTo(url string, payload map[string]interface{}) (int, string, error) {
	return sendTo("PATCH", url, payload)
}

func sendTo(method, url string, payload map[string]interface{}) (int, string, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return -1, "", err
	}

	var req *http.Request

	if payload != nil {
		req, err = http.NewRequestWithContext(context.Background(), method, url, bytes.NewBuffer(payloadJSON))
		if err != nil {
			return -1, "", err
		}
	} else {
		req, err = http.NewRequestWithContext(context.Background(), method, url, nil)
		if err != nil {
			return -1, "", err
		}
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return -1, "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return -1, "", err
	}

	return resp.StatusCode, string(body), nil
}

func validateResp(statusCode int, body string, err error) {
	if err != nil {
		panic(err)
	}
	if statusCode != 200 {
		fmt.Printf("Request failed with status code %d\n", statusCode)
		panic(body)
	}
}

func generateNMembers(amount int) string {
	config, err := config.GetDefaultConfig("../config/default.yaml")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	client := service.NewService(
		database.NewRedisDatabase(database.RedisOptions{
			ClusterEnabled: config.GetBool("redis.clusterEnabled"),
			Addrs:          config.GetStringSlice("redis.addrs"),
			Host:           config.GetString("redis.host"),
			Port:           config.GetInt("redis.port"),
			Password:       config.GetString("redis.password"),
			DB:             config.GetInt("redis.db"),
		}),
	)

	lbID := "leaderboard-0"

	for i := 0; i < amount; i++ {
		if _, err := client.SetMemberScore(context.Background(), lbID, fmt.Sprintf("bench-member-%d", i), int64(100+i), false, "inf"); err != nil {
			panic(err)
		}
	}

	return lbID
}
