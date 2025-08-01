package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/skaronator/lancache-dns-sync/internal/types"
)

type AdguardClient interface {
	GetCurrentRewrites(ctx context.Context) (map[string]string, error)
	AddRewrite(ctx context.Context, domain, answer string) error
	DeleteRewrite(ctx context.Context, domain, answer string) error
}

type HTTPAdguardClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

func NewAdguardClient(baseURL, username, password string, timeout time.Duration) AdguardClient {
	return &HTTPAdguardClient{
		baseURL:  baseURL,
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *HTTPAdguardClient) makeRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	return resp, nil
}

func (c *HTTPAdguardClient) GetCurrentRewrites(ctx context.Context) (map[string]string, error) {
	resp, err := c.makeRequest(ctx, "GET", "/control/rewrite/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get rewrites: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get rewrites: status %d", resp.StatusCode)
	}

	var rewrites []types.DNSRewrite
	if err := json.NewDecoder(resp.Body).Decode(&rewrites); err != nil {
		return nil, fmt.Errorf("failed to decode rewrites response: %w", err)
	}

	result := make(map[string]string)
	for _, rewrite := range rewrites {
		result[rewrite.Domain] = rewrite.Answer
	}

	return result, nil
}

func (c *HTTPAdguardClient) AddRewrite(ctx context.Context, domain, answer string) error {
	rewrite := types.DNSRewrite{Domain: domain, Answer: answer}
	jsonData, err := json.Marshal(rewrite)
	if err != nil {
		return fmt.Errorf("failed to marshal rewrite: %w", err)
	}

	resp, err := c.makeRequest(ctx, "POST", "/control/rewrite/add", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to add rewrite: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to add rewrite for %s: status %d", domain, resp.StatusCode)
	}

	return nil
}

func (c *HTTPAdguardClient) DeleteRewrite(ctx context.Context, domain, answer string) error {
	rewrite := map[string]string{"domain": domain, "answer": answer}
	jsonData, err := json.Marshal(rewrite)
	if err != nil {
		return fmt.Errorf("failed to marshal delete request: %w", err)
	}

	resp, err := c.makeRequest(ctx, "POST", "/control/rewrite/delete", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to delete rewrite: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete rewrite for %s: status %d", domain, resp.StatusCode)
	}

	return nil
}