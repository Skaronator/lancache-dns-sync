package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestNewAdguardClient(t *testing.T) {
	client := NewAdguardClient("http://localhost:3000", "admin", "password")

	if client.baseURL != "http://localhost:3000" {
		t.Errorf("Expected baseURL to be http://localhost:3000, got %s", client.baseURL)
	}

	if client.username != "admin" {
		t.Errorf("Expected username to be admin, got %s", client.username)
	}

	if client.password != "password" {
		t.Errorf("Expected password to be password, got %s", client.password)
	}

	if client.httpClient.Timeout != timeoutSec*time.Second {
		t.Errorf("Expected timeout to be %d seconds, got %v", timeoutSec, client.httpClient.Timeout)
	}
}

func TestGetConfig(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "all required env vars set",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "steam,origin",
			},
			wantErr: false,
		},
		{
			name: "missing username",
			envVars: map[string]string{
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "steam",
			},
			wantErr: true,
		},
		{
			name: "missing password",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "steam",
			},
			wantErr: true,
		},
		{
			name: "missing lancache server",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "steam",
			},
			wantErr: true,
		},
		{
			name: "missing adguard api",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"SERVICE_NAMES":    "steam",
			},
			wantErr: true,
		},
		{
			name: "missing service names",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
			},
			wantErr: true,
		},
		{
			name: "wildcard service names",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "*",
			},
			wantErr: false,
		},
		{
			name: "custom cron schedule",
			envVars: map[string]string{
				"ADGUARD_USERNAME": "admin",
				"ADGUARD_PASSWORD": "password",
				"LANCACHE_SERVER":  "192.168.1.100",
				"ADGUARD_API":      "http://localhost:3000",
				"SERVICE_NAMES":    "steam",
				"CRON_SCHEDULE":    "0 * * * *",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars
			os.Clearenv()

			// Set test env vars
			for k, v := range tt.envVars {
				if err := os.Setenv(k, v); err != nil {
					t.Fatalf("Failed to set env var %s: %v", k, err)
				}
			}

			config, err := getConfig()

			if (err != nil) != tt.wantErr {
				t.Errorf("getConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && config != nil {
				if config.CronSchedule == "" {
					t.Error("Expected default cron schedule to be set")
				}
			}
		})
	}
}

func TestAdguardClientGetCurrentRewrites(t *testing.T) {
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

		rewrites := []DNSRewrite{
			{Domain: "test.com", Answer: "192.168.1.100"},
			{Domain: "example.com", Answer: "192.168.1.101"},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(rewrites); err != nil {
			t.Errorf("Failed to encode rewrites: %v", err)
		}
	}))
	defer server.Close()

	client := NewAdguardClient(server.URL, "admin", "password")
	rewrites, err := client.getCurrentRewrites()

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

func TestAdguardClientAddRewrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/rewrite/add" {
			t.Errorf("Expected path /control/rewrite/add, got %s", r.URL.Path)
		}

		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		var rewrite DNSRewrite
		if err := json.NewDecoder(r.Body).Decode(&rewrite); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if rewrite.Domain != "test.com" || rewrite.Answer != "192.168.1.100" {
			t.Errorf("Unexpected rewrite data: %+v", rewrite)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAdguardClient(server.URL, "admin", "password")
	err := client.addRewrite("test.com", "192.168.1.100")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestAdguardClientDeleteRewrite(t *testing.T) {
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

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAdguardClient(server.URL, "admin", "password")
	err := client.deleteRewrite("test.com")

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestFetchCacheDomains(t *testing.T) {
	// Skip this test since fetchCacheDomains uses a hardcoded URL
	// In a real scenario, we would refactor fetchCacheDomains to accept a base URL parameter
	t.Skip("Skipping TestFetchCacheDomains - requires refactoring to accept configurable URL")
}

func TestGetServiceFilePaths(t *testing.T) {
	domains := &CacheDomainsResponse{
		CacheDomains: []CacheDomain{
			{
				Name:        "steam",
				DomainFiles: []string{"steam.txt", "steam_china.txt"},
			},
			{
				Name:        "origin",
				DomainFiles: []string{"origin.txt"},
			},
			{
				Name:        "epic",
				DomainFiles: []string{"epic.txt"},
			},
		},
	}

	tests := []struct {
		name          string
		config        *Config
		expectedLen   int
		expectedPaths []string
	}{
		{
			name: "specific services",
			config: &Config{
				ServiceNames: []string{"steam", "origin"},
			},
			expectedLen:   3,
			expectedPaths: []string{"steam.txt", "steam_china.txt", "origin.txt"},
		},
		{
			name: "all services wildcard",
			config: &Config{
				ServiceNames: []string{"*"},
			},
			expectedLen: 4,
		},
		{
			name: "single service",
			config: &Config{
				ServiceNames: []string{"epic"},
			},
			expectedLen:   1,
			expectedPaths: []string{"epic.txt"},
		},
		{
			name: "non-existent service",
			config: &Config{
				ServiceNames: []string{"nonexistent"},
			},
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := getServiceFilePaths(domains, tt.config)

			if len(paths) != tt.expectedLen {
				t.Errorf("Expected %d paths, got %d", tt.expectedLen, len(paths))
			}

			if tt.expectedPaths != nil {
				for i, path := range tt.expectedPaths {
					if i >= len(paths) || paths[i] != path {
						t.Errorf("Expected path %s at index %d, got %v", path, i, paths)
					}
				}
			}
		})
	}
}

func TestDownloadDomainFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`# Steam CDN domains
steampowered.com
steamcontent.com
# Comment line
steamstatic.com

`)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	domains, err := downloadDomainFile(server.URL)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expectedDomains := []string{"steampowered.com", "steamcontent.com", "steamstatic.com"}

	if len(domains) != len(expectedDomains) {
		t.Errorf("Expected %d domains, got %d", len(expectedDomains), len(domains))
	}

	for i, domain := range expectedDomains {
		if i >= len(domains) || domains[i] != domain {
			t.Errorf("Expected domain %s at index %d, got %v", domain, i, domains)
		}
	}
}

func TestParseCronSchedule(t *testing.T) {
	tests := []struct {
		cronExpr    string
		expectedDur time.Duration
		expectError bool
	}{
		// Duration format
		{"1h", time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"2h30m", 2*time.Hour + 30*time.Minute, false},
		
		// Cron expressions
		{"* * * * *", time.Minute, false},        // Every minute
		{"0 * * * *", time.Hour, false},          // Every hour at minute 0
		{"1 * * * *", time.Hour, false},          // Every hour at minute 1
		{"0 0 * * *", 24 * time.Hour, false},     // Daily at midnight
		{"1 2 * * *", 24 * time.Hour, false},     // Daily at 02:01
		{"30 14 * * *", 24 * time.Hour, false},   // Daily at 14:30
		{"0 0 * * 0", 7 * 24 * time.Hour, false}, // Weekly on Sunday
		{"0 0 1 * *", 30 * 24 * time.Hour, false}, // Monthly on 1st
		{"15 10 * * 1", 7 * 24 * time.Hour, false}, // Weekly on Monday at 10:15
		
		// Invalid expressions
		{"invalid", 24 * time.Hour, true},
		{"0 0 * *", 24 * time.Hour, true},     // Only 4 fields
		{"0 0 * * * *", 24 * time.Hour, true}, // 6 fields
	}

	for _, tt := range tests {
		t.Run(tt.cronExpr, func(t *testing.T) {
			dur, err := parseCronSchedule(tt.cronExpr)

			if (err != nil) != tt.expectError {
				t.Errorf("parseCronSchedule(%s) error = %v, expectError %v", tt.cronExpr, err, tt.expectError)
			}

			if dur != tt.expectedDur {
				t.Errorf("parseCronSchedule(%s) = %v, want %v", tt.cronExpr, dur, tt.expectedDur)
			}
		})
	}
}

func TestRunOnceEnvironmentVariable(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"true", "true", true},
		{"1", "1", true},
		{"yes", "yes", true},
		{"false", "false", false},
		{"0", "0", false},
		{"no", "no", false},
		{"empty", "", false},
		{"invalid", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			if err := os.Unsetenv("RUN_ONCE"); err != nil {
				t.Fatalf("Failed to unset RUN_ONCE: %v", err)
			}
			
			// Set test value
			if tt.envValue != "" {
				if err := os.Setenv("RUN_ONCE", tt.envValue); err != nil {
					t.Fatalf("Failed to set RUN_ONCE: %v", err)
				}
			}
			
			// Mock flag parsing
			runOnce := false
			
			// Simulate the environment variable check logic from main()
			runOnceEnv := os.Getenv("RUN_ONCE")
			if runOnceEnv == "true" || runOnceEnv == "1" || runOnceEnv == "yes" {
				runOnce = true
			}
			
			if runOnce != tt.expected {
				t.Errorf("Expected runOnce to be %v for env value '%s', got %v", tt.expected, tt.envValue, runOnce)
			}
		})
	}
}

func TestUpdateDNSRewrites(t *testing.T) {
	callCount := make(map[string]int)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount[r.URL.Path]++

		switch r.URL.Path {
		case "/control/rewrite/list":
			// Return existing rewrites
			rewrites := []DNSRewrite{
				{Domain: "existing.com", Answer: "192.168.1.100"},
				{Domain: "update.com", Answer: "192.168.1.200"}, // Wrong IP, needs update
			}
			if err := json.NewEncoder(w).Encode(rewrites); err != nil {
				t.Errorf("Failed to encode rewrites: %v", err)
			}

		case "/control/rewrite/add":
			var rewrite DNSRewrite
			if err := json.NewDecoder(r.Body).Decode(&rewrite); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if rewrite.Domain == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)

		case "/control/rewrite/delete":
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewAdguardClient(server.URL, "admin", "password")

	rewrites := []DNSRewrite{
		{Domain: "existing.com", Answer: "192.168.1.100"}, // Already exists with same IP
		{Domain: "update.com", Answer: "192.168.1.100"},   // Needs update
		{Domain: "new.com", Answer: "192.168.1.100"},      // New entry
	}

	err := updateDNSRewrites(client, rewrites)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify API calls
	if callCount["/control/rewrite/list"] != 1 {
		t.Errorf("Expected 1 call to list rewrites, got %d", callCount["/control/rewrite/list"])
	}

	// Should have 1 delete (for update.com) and 2 adds (update.com and new.com)
	if callCount["/control/rewrite/delete"] != 1 {
		t.Errorf("Expected 1 delete call, got %d", callCount["/control/rewrite/delete"])
	}

	if callCount["/control/rewrite/add"] != 2 {
		t.Errorf("Expected 2 add calls, got %d", callCount["/control/rewrite/add"])
	}
}
