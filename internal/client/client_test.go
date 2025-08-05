package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/skaronator/lancache-dns-sync/internal/types"
)

func TestNewAdguardClient(t *testing.T) {
	timeout := 30 * time.Second
	client := NewAdguardClient("http://localhost:3000", "admin", "password", timeout)

	httpClient, ok := client.(*HTTPAdguardClient)
	if !ok {
		t.Fatal("Expected HTTPAdguardClient")
	}

	if httpClient.baseURL != "http://localhost:3000" {
		t.Errorf("Expected baseURL to be http://localhost:3000, got %s", httpClient.baseURL)
	}

	if httpClient.username != "admin" {
		t.Errorf("Expected username to be admin, got %s", httpClient.username)
	}

	if httpClient.password != "password" {
		t.Errorf("Expected password to be password, got %s", httpClient.password)
	}

	if httpClient.httpClient.Timeout != timeout {
		t.Errorf("Expected timeout to be %v, got %v", timeout, httpClient.httpClient.Timeout)
	}
}

func TestHTTPAdguardClientGetFilteringStatus(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/filtering/status" {
			t.Errorf("Expected path /control/filtering/status, got %s", r.URL.Path)
		}

		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		// Check basic auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "admin" || password != "password" {
			t.Error("Expected valid basic auth")
		}

		status := types.FilterStatus{
			UserRules: []string{
				"||example.com^$dnsrewrite=NOERROR;A;192.168.1.100",
				"# lancache-dns-sync start",
				"|test.com^$dnsrewrite=NOERROR;A;192.168.1.101,important",
				"# lancache-dns-sync end",
				"||custom.com^",
			},
			Enabled: true,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			t.Errorf("Failed to encode status: %v", err)
		}
	}))
	defer server.Close()

	client := NewAdguardClient(server.URL, "admin", "password", 30*time.Second)
	status, err := client.GetFilteringStatus(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !status.Enabled {
		t.Error("Expected filtering to be enabled")
	}

	if len(status.UserRules) != 5 {
		t.Errorf("Expected 5 rules, got %d", len(status.UserRules))
	}

	if status.UserRules[0] != "||example.com^$dnsrewrite=NOERROR;A;192.168.1.100" {
		t.Errorf("Expected first rule to be ||example.com^$dnsrewrite=NOERROR;A;192.168.1.100, got %s", status.UserRules[0])
	}
}

func TestHTTPAdguardClientSetFilteringRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/filtering/set_rules" {
			t.Errorf("Expected path /control/filtering/set_rules, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		var request types.SetRulesRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if len(request.Rules) != 3 {
			t.Errorf("Expected 3 rules, got %d", len(request.Rules))
		}

		expectedRules := []string{
			"|example.com^$dnsrewrite=NOERROR;A;192.168.1.100,important",
			"|test.com^$dnsrewrite=NOERROR;A;192.168.1.101,important",
			"# comment",
		}

		for i, rule := range expectedRules {
			if request.Rules[i] != rule {
				t.Errorf("Expected rule %d to be %s, got %s", i, rule, request.Rules[i])
			}
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAdguardClient(server.URL, "admin", "password", 30*time.Second)
	rules := []string{
		"|example.com^$dnsrewrite=NOERROR;A;192.168.1.100,important",
		"|test.com^$dnsrewrite=NOERROR;A;192.168.1.101,important",
		"# comment",
	}
	err := client.SetFilteringRules(context.Background(), rules)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}
