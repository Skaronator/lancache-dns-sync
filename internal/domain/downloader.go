package domain

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/skaronator/lancache-dns-sync/internal/config"
	"github.com/skaronator/lancache-dns-sync/internal/types"
)

const (
	BaseURL         = "https://raw.githubusercontent.com/uklans/cache-domains/master/"
	JSONPath        = "cache_domains.json"
	MaxConcurrency  = 10
)

type Downloader struct {
	httpClient  *http.Client
	baseURL     string
	concurrency int
}

func NewDownloader(httpClient *http.Client) *Downloader {
	return &Downloader{
		httpClient:  httpClient,
		baseURL:     BaseURL,
		concurrency: MaxConcurrency,
	}
}

func (d *Downloader) FetchCacheDomains(ctx context.Context) (*types.CacheDomainsResponse, error) {
	url := d.baseURL + JSONPath
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch cache domains: status %d", resp.StatusCode)
	}

	var result types.CacheDomainsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (d *Downloader) GetServiceFilePaths(domains *types.CacheDomainsResponse, cfg *config.Config) []string {
	var filePaths []string

	if cfg.IsAllServices() {
		for _, domain := range domains.CacheDomains {
			filePaths = append(filePaths, domain.DomainFiles...)
		}
		return filePaths
	}

	// Create a map of available services for quick lookup
	availableServices := make(map[string]bool)
	for _, domain := range domains.CacheDomains {
		availableServices[domain.Name] = true
	}

	// Check each requested service and warn if not found
	for _, serviceName := range cfg.ServiceNames {
		if !availableServices[serviceName] {
			slog.Warn("Requested service not found in cache domains", "service", serviceName)
		}
	}

	// Collect file paths for existing services
	for _, domain := range domains.CacheDomains {
		if cfg.HasService(domain.Name) {
			filePaths = append(filePaths, domain.DomainFiles...)
		}
	}

	return filePaths
}

func (d *Downloader) downloadDomainFile(ctx context.Context, url string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download %s: status %d", url, resp.StatusCode)
	}

	var domains []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			domains = append(domains, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan response body: %w", err)
	}

	return domains, nil
}

func (d *Downloader) DownloadDomainsFromFiles(ctx context.Context, filePaths []string, lancacheServer string) ([]types.DNSRewrite, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allRewrites []types.DNSRewrite

	semaphore := make(chan struct{}, d.concurrency)

	for _, filePath := range filePaths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			url := d.baseURL + path
			domains, err := d.downloadDomainFile(ctx, url)
			if err != nil {
				slog.Error("Error downloading domain file", "url", url, "error", err)
				return
			}

			var rewrites []types.DNSRewrite
			for _, domain := range domains {
				rewrites = append(rewrites, types.DNSRewrite{
					Domain: domain,
					Answer: lancacheServer,
				})
			}

			mu.Lock()
			allRewrites = append(allRewrites, rewrites...)
			mu.Unlock()
		}(filePath)
	}

	wg.Wait()
	return allRewrites, nil
}