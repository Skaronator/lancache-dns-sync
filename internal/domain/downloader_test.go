package domain

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/skaronator/lancache-dns-sync/internal/config"
	"github.com/skaronator/lancache-dns-sync/internal/types"
)

func TestGetServiceFilePaths(t *testing.T) {
	downloader := NewDownloader(&http.Client{Timeout: 30 * time.Second})

	domains := &types.CacheDomainsResponse{
		CacheDomains: []types.CacheDomain{
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
		config        *config.Config
		expectedLen   int
		expectedPaths []string
	}{
		{
			name: "specific services",
			config: &config.Config{
				ServiceNames: []string{"steam", "origin"},
			},
			expectedLen:   3,
			expectedPaths: []string{"steam.txt", "steam_china.txt", "origin.txt"},
		},
		{
			name: "all services wildcard",
			config: &config.Config{
				ServiceNames: []string{"*"},
			},
			expectedLen: 4,
		},
		{
			name: "single service",
			config: &config.Config{
				ServiceNames: []string{"epic"},
			},
			expectedLen:   1,
			expectedPaths: []string{"epic.txt"},
		},
		{
			name: "non-existent service",
			config: &config.Config{
				ServiceNames: []string{"nonexistent"},
			},
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := downloader.GetServiceFilePaths(domains, tt.config)

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

	downloader := NewDownloader(&http.Client{Timeout: 30 * time.Second})
	domains, err := downloader.downloadDomainFile(context.Background(), server.URL)

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

func TestGetServiceFilePathsWithWarnings(t *testing.T) {
	downloader := NewDownloader(&http.Client{Timeout: 30 * time.Second})

	domains := &types.CacheDomainsResponse{
		CacheDomains: []types.CacheDomain{
			{
				Name:        "steam",
				DomainFiles: []string{"steam.txt"},
			},
			{
				Name:        "origin",
				DomainFiles: []string{"origin.txt"},
			},
		},
	}

	// Test with mix of valid and invalid service names
	config := &config.Config{
		ServiceNames: []string{"steam", "nonexistent", "origin", "invalid"},
	}

	paths := downloader.GetServiceFilePaths(domains, config)

	// Should return paths for valid services only
	expectedLen := 2 // steam.txt + origin.txt
	if len(paths) != expectedLen {
		t.Errorf("Expected %d paths, got %d", expectedLen, len(paths))
	}

	// Should contain valid service files
	expectedPaths := map[string]bool{"steam.txt": false, "origin.txt": false}
	for _, path := range paths {
		if _, exists := expectedPaths[path]; exists {
			expectedPaths[path] = true
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("Expected path %s not found in results", path)
		}
	}
}

func TestFetchCacheDomains(t *testing.T) {
	tests := []struct {
		name          string
		responseBody  string
		statusCode    int
		expectError   bool
		expectedCount int
	}{
		{
			name: "successful fetch",
			responseBody: `{
				"cache_domains": [
					{
						"name": "steam",
						"description": "Steam CDN",
						"domain_files": ["steam.txt", "steam_china.txt"]
					},
					{
						"name": "origin",
						"description": "Origin CDN",
						"domain_files": ["origin.txt"]
					}
				]
			}`,
			statusCode:    200,
			expectError:   false,
			expectedCount: 2,
		},
		{
			name:         "server error",
			responseBody: "Internal Server Error",
			statusCode:   500,
			expectError:  true,
		},
		{
			name:         "invalid json",
			responseBody: `{"invalid": json}`,
			statusCode:   200,
			expectError:  true,
		},
		{
			name:          "empty response",
			responseBody:  `{"cache_domains": []}`,
			statusCode:    200,
			expectError:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if _, err := w.Write([]byte(tt.responseBody)); err != nil {
					t.Errorf("Failed to write response: %v", err)
				}
			}))
			defer server.Close()

			downloader := NewDownloader(&http.Client{Timeout: 30 * time.Second})
			downloader.baseURL = server.URL + "/"

			result, err := downloader.FetchCacheDomains(context.Background())

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError && result != nil {
				if len(result.CacheDomains) != tt.expectedCount {
					t.Errorf("Expected %d cache domains, got %d", tt.expectedCount, len(result.CacheDomains))
				}
			}
		})
	}
}

func TestDownloadDomainsFromFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/steam.txt":
			if _, err := w.Write([]byte(`steampowered.com
steamcontent.com
steamstatic.com`)); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		case "/origin.txt":
			if _, err := w.Write([]byte(`origin.com
ea.com`)); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		case "/error.txt":
			w.WriteHeader(404)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	downloader := NewDownloader(&http.Client{Timeout: 30 * time.Second})
	downloader.baseURL = server.URL + "/"

	tests := []struct {
		name            string
		filePaths       []string
		lancacheServer  string
		expectedCount   int
		expectedDomains []string
	}{
		{
			name:           "download multiple files",
			filePaths:      []string{"steam.txt", "origin.txt"},
			lancacheServer: "192.168.1.100",
			expectedCount:  5,
			expectedDomains: []string{
				"steampowered.com", "steamcontent.com", "steamstatic.com",
				"origin.com", "ea.com",
			},
		},
		{
			name:           "download single file",
			filePaths:      []string{"steam.txt"},
			lancacheServer: "192.168.1.100",
			expectedCount:  3,
			expectedDomains: []string{
				"steampowered.com", "steamcontent.com", "steamstatic.com",
			},
		},
		{
			name:           "no files",
			filePaths:      []string{},
			lancacheServer: "192.168.1.100",
			expectedCount:  0,
		},
		{
			name:           "file with error is skipped",
			filePaths:      []string{"steam.txt", "error.txt"},
			lancacheServer: "192.168.1.100",
			expectedCount:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rewrites, err := downloader.DownloadDomainsFromFiles(context.Background(), tt.filePaths, tt.lancacheServer)

			if err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if len(rewrites) != tt.expectedCount {
				t.Errorf("Expected %d rewrites, got %d", tt.expectedCount, len(rewrites))
			}

			domainMap := make(map[string]bool)
			for _, rewrite := range rewrites {
				domainMap[rewrite.Domain] = true
				if rewrite.Answer != tt.lancacheServer {
					t.Errorf("Expected answer %s, got %s for domain %s", tt.lancacheServer, rewrite.Answer, rewrite.Domain)
				}
			}

			for _, expectedDomain := range tt.expectedDomains {
				if !domainMap[expectedDomain] {
					t.Errorf("Expected domain %s not found in results", expectedDomain)
				}
			}
		})
	}
}

func TestDownloadDomainFileErrors(t *testing.T) {
	tests := []struct {
		name        string
		serverFunc  func(w http.ResponseWriter, r *http.Request)
		expectError bool
	}{
		{
			name: "404 not found",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(404)
			},
			expectError: true,
		},
		{
			name: "500 server error",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverFunc))
			defer server.Close()

			downloader := NewDownloader(&http.Client{Timeout: 1 * time.Second})

			_, err := downloader.downloadDomainFile(context.Background(), server.URL)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestDownloadDomainFileWithComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(`# This is a comment
steampowered.com
# Another comment
steamcontent.com

# Empty line above should be ignored
steamstatic.com
`)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	downloader := NewDownloader(&http.Client{Timeout: 30 * time.Second})
	domains, err := downloader.downloadDomainFile(context.Background(), server.URL)

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
