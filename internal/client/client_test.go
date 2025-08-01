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

func TestHTTPAdguardClientGetCurrentRewrites(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/rewrite/list" {
			t.Errorf("Expected path /control/rewrite/list, got %s", r.URL.Path)
		}

		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		// Check basic auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "admin" || password != "password" {
			t.Error("Expected valid basic auth")
		}

		rewrites := []types.DNSRewrite{
			{Domain: "test.com", Answer: "192.168.1.100"},
			{Domain: "example.com", Answer: "192.168.1.101"},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(rewrites); err != nil {
			t.Errorf("Failed to encode rewrites: %v", err)
		}
	}))
	defer server.Close()

	client := NewAdguardClient(server.URL, "admin", "password", 30*time.Second)
	rewrites, err := client.GetCurrentRewrites(context.Background())

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(rewrites) != 2 {
		t.Errorf("Expected 2 rewrites, got %d", len(rewrites))
	}

	if rewrites["test.com"] != "192.168.1.100" {
		t.Errorf("Expected test.com to map to 192.168.1.100, got %s", rewrites["test.com"])
	}
}

func TestHTTPAdguardClientAddRewrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/rewrite/add" {
			t.Errorf("Expected path /control/rewrite/add, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		var rewrite types.DNSRewrite
		if err := json.NewDecoder(r.Body).Decode(&rewrite); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if rewrite.Domain != "test.com" || rewrite.Answer != "192.168.1.100" {
			t.Errorf("Unexpected rewrite data: %+v", rewrite)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAdguardClient(server.URL, "admin", "password", 30*time.Second)
	err := client.AddRewrite(context.Background(), "test.com", "192.168.1.100")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestHTTPAdguardClientDeleteRewrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/rewrite/delete" {
			t.Errorf("Expected path /control/rewrite/delete, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		var data map[string]string
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if data["domain"] != "test.com" {
			t.Errorf("Expected domain test.com, got %s", data["domain"])
		}

		if data["answer"] != "192.168.1.100" {
			t.Errorf("Expected answer 192.168.1.100, got %s", data["answer"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAdguardClient(server.URL, "admin", "password", 30*time.Second)
	err := client.DeleteRewrite(context.Background(), "test.com", "192.168.1.100")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}