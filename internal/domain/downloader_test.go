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