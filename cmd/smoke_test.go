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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if request.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", request.Method)
		}
		if string(body) != `{"score":10}` {
			t.Errorf("unexpected request body: %s", body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	defer server.Close()

	previousBaseURL := baseURL
	baseURL = server.URL
	t.Cleanup(func() {
		baseURL = previousBaseURL
	})

	status, body, err := doRequest(http.MethodPost, "/scores", `{"score":10}`)
	if err != nil {
		t.Fatalf("make request: %v", err)
	}
	if status != http.StatusCreated || body != "created" {
		t.Fatalf("expected 201 created, got %d %q", status, body)
	}
}

func TestDoRequestWithoutBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Body != http.NoBody {
			t.Errorf("expected an empty request body")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	previousBaseURL := baseURL
	baseURL = server.URL
	t.Cleanup(func() {
		baseURL = previousBaseURL
	})

	status, body, err := doRequest(http.MethodGet, "/healthcheck", "")
	if err != nil {
		t.Fatalf("make request: %v", err)
	}
	if status != http.StatusNoContent || body != "" {
		t.Fatalf("expected 204 with an empty body, got %d %q", status, body)
	}
}

func TestDoRequestErrors(t *testing.T) {
	previousBaseURL := baseURL
	t.Cleanup(func() {
		baseURL = previousBaseURL
	})

	baseURL = "http://[::1"
	if _, _, err := doRequest(http.MethodGet, "/", ""); err == nil {
		t.Fatal("expected invalid URL to return an error")
	}

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL = server.URL
	server.Close()
	if _, _, err := doRequest(http.MethodGet, "/", ""); err == nil {
		t.Fatal("expected connection failure to return an error")
	}
}

func TestSmokeCommand(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requestCount++
		if request.URL.Path == "/healthcheck" {
			if request.Method != http.MethodGet {
				t.Errorf("expected health check to use GET, got %s", request.Method)
			}
			_, _ = w.Write([]byte("WORKING"))
			return
		}

		switch {
		case strings.HasSuffix(request.URL.Path, "/score"):
			if request.Method != http.MethodPut {
				t.Errorf("expected score update to use PUT, got %s", request.Method)
			}
		case request.URL.Path == "/l":
			t.Errorf("expected a leaderboard identifier in path %q", request.URL.Path)
		case strings.Count(request.URL.Path, "/") == 2:
			if request.Method != http.MethodDelete {
				t.Errorf("expected leaderboard removal to use DELETE, got %s", request.Method)
			}
		}
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	previousBaseURL := baseURL
	baseURL = server.URL
	t.Cleanup(func() {
		baseURL = previousBaseURL
	})

	if err := smokeCmd.RunE(smokeCmd, nil); err != nil {
		t.Fatalf("run smoke command: %v", err)
	}

	const expectedRequests = 506
	if requestCount != expectedRequests {
		t.Fatalf("expected %d smoke-test requests, got %d", expectedRequests, requestCount)
	}
}

func TestSmokeOperationsReturnHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	previousBaseURL := baseURL
	baseURL = server.URL
	t.Cleanup(func() {
		baseURL = previousBaseURL
	})

	tests := []struct {
		name string
		call func() error
	}{
		{name: "health check", call: doHealthCheck},
		{name: "add score", call: func() error { return addMemberScore("board", "member", 10) }},
		{name: "get member", call: func() error { return getMember("board", "member") }},
		{name: "get members", call: func() error { return getMembers("board", "member") }},
		{name: "get rank", call: func() error { return getRank("board", "member") }},
		{name: "get around", call: func() error { return getAround("board", "member") }},
		{name: "get member count", call: func() error { return getNumberOfMembers("board") }},
		{name: "get top", call: func() error { return getTopMembers("board") }},
		{name: "get percentage", call: func() error { return getTopPercentage("board") }},
		{name: "remove member", call: func() error { return removeMember("board", "member") }},
		{name: "remove leaderboard", call: func() error { return removeLeaderboard("board") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatal("expected failed HTTP response to return an error")
			}
		})
	}
}

func TestSmokeOperationsReturnConnectionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	previousBaseURL := baseURL
	baseURL = server.URL
	server.Close()
	t.Cleanup(func() {
		baseURL = previousBaseURL
	})

	if err := doHealthCheck(); err == nil {
		t.Fatal("expected health check connection error")
	}
	if err := getMember("board", "member"); err == nil {
		t.Fatal("expected operation connection error")
	}
}
