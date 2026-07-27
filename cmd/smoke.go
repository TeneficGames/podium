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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var baseURL string

var smokeHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

func doRequest(method, url, reqBody string) (int, string, error) {
	absURL := fmt.Sprintf("%s%s", baseURL, url)

	var req *http.Request
	var err error
	if reqBody != "" {
		req, err = http.NewRequestWithContext(context.Background(), method, absURL, bytes.NewBufferString(reqBody))
	} else {
		req, err = http.NewRequestWithContext(context.Background(), method, absURL, nil)
	}
	if err != nil {
		return http.StatusInternalServerError, "", err
	}

	response, err := smokeHTTPClient.Do(req)
	if err != nil {
		return 500, "", err
	}
	defer func() {
		_ = response.Body.Close()
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return 500, "", err
	}
	return response.StatusCode, string(body), nil
}

// smokeCmd represents the smoke command
var smokeCmd = &cobra.Command{
	Use:   "smoke",
	Short: "performs a smoke test",
	Long: `Runs a smoke test in a given instance of podium.
A smoke test will perform all the available operations in a leaderboard and then remove it.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := doHealthCheck(); err != nil {
			return err
		}

		leaderboardID := uuid.New().String()
		fmt.Printf("Creating leaderboard %s...\n\n", leaderboardID)

		fmt.Println("Adding member scores to leaderboard...")
		for i := 0; i < 100; i++ {
			if err := addMemberScore(leaderboardID, fmt.Sprintf("member-%d", i), 100-i); err != nil {
				return err
			}
		}
		fmt.Println("Member scores added to leaderboard successfully.")

		fmt.Println("Getting member details from leaderboard...")
		for i := 0; i < 100; i++ {
			if err := getMember(leaderboardID, fmt.Sprintf("member-%d", i)); err != nil {
				return err
			}
		}
		fmt.Println("Member details retrieved successfully.")

		fmt.Println("Getting many members from leaderboard...")
		memberIDs := []string{}
		for i := 0; i < 100; i++ {
			memberIDs = append(memberIDs, fmt.Sprintf("member-%d", i))
		}
		if err := getMembers(leaderboardID, strings.Join(memberIDs, ",")); err != nil {
			return err
		}
		fmt.Println("Members retrieved successfully.")

		fmt.Println("Getting members ranks from leaderboard...")
		for i := 0; i < 100; i++ {
			if err := getRank(leaderboardID, fmt.Sprintf("member-%d", i)); err != nil {
				return err
			}
		}
		fmt.Println("Members ranks retrieved successfully.")

		fmt.Println("Getting members around a member from leaderboard...")
		for i := 0; i < 100; i++ {
			if err := getAround(leaderboardID, fmt.Sprintf("member-%d", i)); err != nil {
				return err
			}
		}
		fmt.Println("Members around a member retrieved successfully.")

		fmt.Println("Getting number of members in a leaderboard...")
		if err := getNumberOfMembers(leaderboardID); err != nil {
			return err
		}
		fmt.Println("Number of members retrieved successfully.")

		fmt.Println("Getting top members in a leaderboard...")
		if err := getTopMembers(leaderboardID); err != nil {
			return err
		}
		fmt.Println("Top members retrieved successfully.")

		fmt.Println("Getting top 5% members in a leaderboard...")
		if err := getTopPercentage(leaderboardID); err != nil {
			return err
		}
		fmt.Println("Top 5% retrieved successfully.")

		fmt.Println("Removing members from leaderboard...")
		for i := 0; i < 100; i++ {
			if err := removeMember(leaderboardID, fmt.Sprintf("member-%d", i)); err != nil {
				return err
			}
		}
		fmt.Println("Members removed successfully.")

		fmt.Println("Removing leaderboard...")
		if err := removeLeaderboard(leaderboardID); err != nil {
			return err
		}
		fmt.Println("Leaderboard removed successfully.")
		return nil
	},
}

func doHealthCheck() error {
	fmt.Println("Starting smoke test...")
	status, body, err := doRequest("GET", "/healthcheck", "")
	if err != nil {
		return fmt.Errorf("could not reach %s: %w", baseURL, err)
	}
	if status != 200 || body != "WORKING" {
		return fmt.Errorf("could not reach %s (status: %d): %s", baseURL, status, body)
	}
	return nil
}

func doOKRequest(method, url, requestBody string) error {
	status, responseBody, err := doRequest(method, url, requestBody)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %d: %s", status, responseBody)
	}
	return nil
}

func addMemberScore(leaderboardID, memberID string, score int) error {
	url := fmt.Sprintf("/l/%s/members/%s/score", leaderboardID, memberID)
	return doOKRequest(
		"PUT",
		url,
		fmt.Sprintf("{\"score\":%d}", score),
	)
}

func getMember(leaderboardID, memberID string) error {
	url := fmt.Sprintf("/l/%s/members/%s", leaderboardID, memberID)
	return doOKRequest(
		"GET",
		url,
		"",
	)
}

func getMembers(leaderboardID, memberIDs string) error {
	url := fmt.Sprintf("/l/%s/members?ids=%s", leaderboardID, memberIDs)
	return doOKRequest(
		"GET",
		url,
		"",
	)
}

func getRank(leaderboardID, memberID string) error {
	url := fmt.Sprintf("/l/%s/members/%s/rank", leaderboardID, memberID)
	return doOKRequest(
		"GET",
		url,
		"",
	)
}

func getAround(leaderboardID, memberID string) error {
	url := fmt.Sprintf("/l/%s/members/%s/around", leaderboardID, memberID)
	return doOKRequest(
		"GET",
		url,
		"",
	)
}

func getNumberOfMembers(leaderboardID string) error {
	url := fmt.Sprintf("/l/%s/members-count", leaderboardID)
	return doOKRequest(
		"GET",
		url,
		"",
	)
}

func getTopMembers(leaderboardID string) error {
	url := fmt.Sprintf("/l/%s/top/1?pageSize=20", leaderboardID)

	return doOKRequest(
		"GET",
		url,
		"",
	)
}

func getTopPercentage(leaderboardID string) error {
	url := fmt.Sprintf("/l/%s/top-percent/5", leaderboardID)

	return doOKRequest(
		"GET",
		url,
		"",
	)
}

func removeMember(leaderboardID, memberID string) error {
	url := fmt.Sprintf("/l/%s/members/%s", leaderboardID, memberID)
	return doOKRequest(
		"DELETE",
		url,
		"",
	)
}

func removeLeaderboard(leaderboardID string) error {
	url := fmt.Sprintf("/l/%s", leaderboardID)
	return doOKRequest(
		"DELETE",
		url,
		"",
	)
}

func init() {
	RootCmd.AddCommand(smokeCmd)
	smokeCmd.Flags().StringVarP(&baseURL, "base-url", "b", "http://localhost:8888", "Base URL for podium.")
}
