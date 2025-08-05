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
	GetFilteringStatus(ctx context.Context) (*types.FilterStatus, error)
	SetFilteringRules(ctx context.Context, rules []string) error
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

func (c *HTTPAdguardClient) GetFilteringStatus(ctx context.Context) (*types.FilterStatus, error) {
	resp, err := c.makeRequest(ctx, "GET", "/control/filtering/status", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get filtering status: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get filtering status: status %d", resp.StatusCode)
	}

	var status types.FilterStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode filtering status response: %w", err)
	}

	return &status, nil
}

func (c *HTTPAdguardClient) SetFilteringRules(ctx context.Context, rules []string) error {
	request := types.SetRulesRequest{Rules: rules}
	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal rules request: %w", err)
	}

	resp, err := c.makeRequest(ctx, "POST", "/control/filtering/set_rules", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to set filtering rules: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Error("Failed to close response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to set filtering rules: status %d", resp.StatusCode)
	}

	return nil
}