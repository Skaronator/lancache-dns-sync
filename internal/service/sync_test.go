package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/skaronator/lancache-dns-sync/internal/config"
	"github.com/skaronator/lancache-dns-sync/internal/domain"
	"github.com/skaronator/lancache-dns-sync/internal/types"
)

type downloaderInterface interface {
	FetchCacheDomains(ctx context.Context) (*types.CacheDomainsResponse, error)
	GetServiceFilePaths(domains *types.CacheDomainsResponse, cfg *config.Config) []string
	DownloadDomainsFromFiles(ctx context.Context, filePaths []string, lancacheServer string) ([]types.DNSRewrite, error)
}

type clientInterface interface {
	GetFilteringStatus(ctx context.Context) (*types.FilterStatus, error)
	SetFilteringRules(ctx context.Context, rules []string) error
}

// Test version of SyncService that uses interface types
type testSyncService struct {
	client     clientInterface
	downloader downloaderInterface
	config     *config.Config
}

func (s *testSyncService) SyncDomains(ctx context.Context) error {
	domains, err := s.downloader.FetchCacheDomains(ctx)
	if err != nil {
		return err
	}

	filePaths := s.downloader.GetServiceFilePaths(domains, s.config)
	if len(filePaths) == 0 {
		return nil
	}

	rewrites, err := s.downloader.DownloadDomainsFromFiles(ctx, filePaths, s.config.LancacheServer.String())
	if err != nil {
		return err
	}

	return s.updateFilteringRules(ctx, rewrites)
}

func (s *testSyncService) updateFilteringRules(ctx context.Context, rewrites []types.DNSRewrite) error {
	status, err := s.client.GetFilteringStatus(ctx)
	if err != nil {
		return err
	}

	existingRules := status.UserRules
	preservedRules := extractNonManagedRules(existingRules)

	newRules := []string{}
	newRules = append(newRules, preservedRules...)
	newRules = append(newRules, startMarker)

	for _, rewrite := range rewrites {
		rule := "|" + rewrite.Domain + "^$dnsrewrite=NOERROR;A;" + rewrite.Answer + ",important"
		newRules = append(newRules, rule)
	}

	newRules = append(newRules, endMarker)

	return s.client.SetFilteringRules(ctx, newRules)
}

func TestExtractNonManagedRules(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "Rules with managed section",
			input: []string{
				"||example.com^",
				"# lancache-dns-sync start",
				"|managed1.com^$dnsrewrite=NOERROR;A;192.168.1.1,important",
				"|managed2.com^$dnsrewrite=NOERROR;A;192.168.1.1,important",
				"# lancache-dns-sync end",
				"||custom.com^",
			},
			expected: []string{
				"||example.com^",
				"||custom.com^",
			},
		},
		{
			name: "No managed section",
			input: []string{
				"||example.com^",
				"||custom.com^",
			},
			expected: []string{
				"||example.com^",
				"||custom.com^",
			},
		},
		{
			name:     "Empty rules",
			input:    []string{},
			expected: []string{},
		},
		{
			name: "Only managed section",
			input: []string{
				"# lancache-dns-sync start",
				"|managed.com^$dnsrewrite=NOERROR;A;192.168.1.1,important",
				"# lancache-dns-sync end",
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractNonManagedRules(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d rules, got %d", len(tt.expected), len(result))
				return
			}

			for i, rule := range result {
				if rule != tt.expected[i] {
					t.Errorf("Expected rule %d to be %s, got %s", i, tt.expected[i], rule)
				}
			}
		})
	}
}

type mockAdguardClient struct {
	filteringStatus *types.FilterStatus
	filteringError  error
	setRulesError   error
	setRulesCalled  bool
	lastRules       []string
}

func (m *mockAdguardClient) GetFilteringStatus(ctx context.Context) (*types.FilterStatus, error) {
	return m.filteringStatus, m.filteringError
}

func (m *mockAdguardClient) SetFilteringRules(ctx context.Context, rules []string) error {
	m.setRulesCalled = true
	m.lastRules = rules
	return m.setRulesError
}

type mockDownloader struct {
	domains       *types.CacheDomainsResponse
	domainsPaths  []string
	rewrites      []types.DNSRewrite
	fetchError    error
	downloadError error
}

func (m *mockDownloader) FetchCacheDomains(ctx context.Context) (*types.CacheDomainsResponse, error) {
	return m.domains, m.fetchError
}

func (m *mockDownloader) GetServiceFilePaths(domains *types.CacheDomainsResponse, cfg *config.Config) []string {
	return m.domainsPaths
}

func (m *mockDownloader) DownloadDomainsFromFiles(ctx context.Context, filePaths []string, lancacheServer string) ([]types.DNSRewrite, error) {
	return m.rewrites, m.downloadError
}

func TestSyncService_SyncDomains(t *testing.T) {
	tests := []struct {
		name              string
		client            *mockAdguardClient
		downloader        *mockDownloader
		expectError       bool
		expectedRuleCount int
	}{
		{
			name: "successful sync",
			client: &mockAdguardClient{
				filteringStatus: &types.FilterStatus{
					UserRules: []string{"||existing.com^"},
				},
			},
			downloader: &mockDownloader{
				domains: &types.CacheDomainsResponse{
					CacheDomains: []types.CacheDomain{
						{Name: "steam", DomainFiles: []string{"steam.txt"}},
					},
				},
				domainsPaths: []string{"steam.txt"},
				rewrites: []types.DNSRewrite{
					{Domain: "steampowered.com", Answer: "192.168.1.1"},
					{Domain: "steamcontent.com", Answer: "192.168.1.1"},
				},
			},
			expectError:       false,
			expectedRuleCount: 5, // existing rule + start marker + 2 rules + end marker
		},
		{
			name: "fetch domains fails",
			client: &mockAdguardClient{
				filteringStatus: &types.FilterStatus{UserRules: []string{}},
			},
			downloader: &mockDownloader{
				fetchError: errors.New("fetch failed"),
			},
			expectError: true,
		},
		{
			name: "no domain files to process",
			client: &mockAdguardClient{
				filteringStatus: &types.FilterStatus{UserRules: []string{}},
			},
			downloader: &mockDownloader{
				domains:      &types.CacheDomainsResponse{},
				domainsPaths: []string{},
			},
			expectError: false,
		},
		{
			name: "download domains fails",
			client: &mockAdguardClient{
				filteringStatus: &types.FilterStatus{UserRules: []string{}},
			},
			downloader: &mockDownloader{
				domains:       &types.CacheDomainsResponse{},
				domainsPaths:  []string{"steam.txt"},
				downloadError: errors.New("download failed"),
			},
			expectError: true,
		},
		{
			name: "get filtering status fails",
			client: &mockAdguardClient{
				filteringError: errors.New("status failed"),
			},
			downloader: &mockDownloader{
				domains:      &types.CacheDomainsResponse{},
				domainsPaths: []string{"steam.txt"},
				rewrites:     []types.DNSRewrite{{Domain: "test.com", Answer: "192.168.1.1"}},
			},
			expectError: true,
		},
		{
			name: "set filtering rules fails",
			client: &mockAdguardClient{
				filteringStatus: &types.FilterStatus{UserRules: []string{}},
				setRulesError:   errors.New("set rules failed"),
			},
			downloader: &mockDownloader{
				domains:      &types.CacheDomainsResponse{},
				domainsPaths: []string{"steam.txt"},
				rewrites:     []types.DNSRewrite{{Domain: "test.com", Answer: "192.168.1.1"}},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				ServiceNames:   []string{"steam"},
				LancacheServer: net.ParseIP("192.168.1.1"),
			}
			// Create a test service instance with mocks
			service := &testSyncService{
				client:     tt.client,
				downloader: tt.downloader,
				config:     cfg,
			}

			err := service.SyncDomains(context.Background())

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError && tt.expectedRuleCount > 0 {
				if !tt.client.setRulesCalled {
					t.Error("Expected SetFilteringRules to be called")
				}
				if len(tt.client.lastRules) != tt.expectedRuleCount {
					t.Errorf("Expected %d rules, got %d", tt.expectedRuleCount, len(tt.client.lastRules))
				}
			}
		})
	}
}

func TestSyncService_updateFilteringRules(t *testing.T) {
	tests := []struct {
		name           string
		existingRules  []string
		rewrites       []types.DNSRewrite
		expectError    bool
		expectedRules  []string
		getStatusError error
		setRulesError  error
	}{
		{
			name:          "add rules to empty list",
			existingRules: []string{},
			rewrites: []types.DNSRewrite{
				{Domain: "test.com", Answer: "192.168.1.1"},
			},
			expectedRules: []string{
				startMarker,
				"|test.com^$dnsrewrite=NOERROR;A;192.168.1.1,important",
				endMarker,
			},
		},
		{
			name: "preserve existing non-managed rules",
			existingRules: []string{
				"||custom.com^",
				startMarker,
				"|old.com^$dnsrewrite=NOERROR;A;192.168.1.1,important",
				endMarker,
				"||another.com^",
			},
			rewrites: []types.DNSRewrite{
				{Domain: "new.com", Answer: "192.168.1.2"},
			},
			expectedRules: []string{
				"||custom.com^",
				"||another.com^",
				startMarker,
				"|new.com^$dnsrewrite=NOERROR;A;192.168.1.2,important",
				endMarker,
			},
		},
		{
			name:           "get status fails",
			getStatusError: errors.New("status error"),
			expectError:    true,
		},
		{
			name:          "set rules fails",
			existingRules: []string{},
			rewrites:      []types.DNSRewrite{{Domain: "test.com", Answer: "192.168.1.1"}},
			setRulesError: errors.New("set error"),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockAdguardClient{
				filteringStatus: &types.FilterStatus{
					UserRules: tt.existingRules,
				},
				filteringError: tt.getStatusError,
				setRulesError:  tt.setRulesError,
			}

			cfg := &config.Config{}
			service := &testSyncService{
				client:     client,
				downloader: nil,
				config:     cfg,
			}

			err := service.updateFilteringRules(context.Background(), tt.rewrites)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError && tt.expectedRules != nil {
				if !client.setRulesCalled {
					t.Error("Expected SetFilteringRules to be called")
				}

				if len(client.lastRules) != len(tt.expectedRules) {
					t.Errorf("Expected %d rules, got %d", len(tt.expectedRules), len(client.lastRules))
				}

				for i, expected := range tt.expectedRules {
					if i >= len(client.lastRules) || client.lastRules[i] != expected {
						t.Errorf("Expected rule %d to be %q, got %q", i, expected, client.lastRules[i])
					}
				}
			}
		})
	}
}

func TestNewSyncService(t *testing.T) {
	client := &mockAdguardClient{}
	downloader := domain.NewDownloader(&http.Client{Timeout: 30 * time.Second})
	cfg := &config.Config{}

	service := NewSyncService(client, downloader, cfg)

	if service == nil {
		t.Fatal("NewSyncService should return a non-nil service")
	}
	if service.config != cfg {
		t.Error("Service config should match provided config")
	}
}
