package proxycheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"dayusch/internal/pkg/api/yarun"

	"github.com/charmbracelet/log"
	"golang.org/x/sync/semaphore"
)

// ProxyChecker handles proxy checking operations
type ProxyChecker struct {
	yarunAPI      *yarun.YarunApi
	limit         int
	testURLs      []string
	httpClient    *http.Client
	proxyUsername *string
	proxyPassword *string
	semaphore     *semaphore.Weighted
}

// IPifyResponse represents the response from ipify API
type IPifyResponse struct {
	IP string `json:"ip"`
}

// NewProxyChecker creates a new proxy checker instance
func NewProxyChecker(yarunURL, yarunToken, proxyUsername, proxyPassword string, limit int) *ProxyChecker {
	return &ProxyChecker{
		yarunAPI: yarun.NewYarunApi(yarunURL, yarunToken),
		limit:    limit,
		testURLs: []string{
			"https://jktjkt48.com",
			"https://idrok5.com",
			"https://idrgamerp.com",
			"https://test.1gvdjbxcw.com",
		},
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		proxyUsername: &proxyUsername,
		proxyPassword: &proxyPassword,
		semaphore:     semaphore.NewWeighted(10), // Allow up to 10 concurrent proxy checks
	}
}

// CheckProxies performs the main proxy checking logic
func (pc *ProxyChecker) CheckProxies(ctx context.Context) error {
	log.Info("Fetching blocked proxies...")

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Get blocked proxies
	blockedResp, err := pc.yarunAPI.GetBlockedProxies(ctx, pc.limit)
	if err != nil {
		return fmt.Errorf("failed to get blocked proxies: %w", err)
	}

	if !blockedResp.Ok {
		return fmt.Errorf("API returned not ok for blocked proxies")
	}

	log.Info("Found blocked proxies to check", "count", len(blockedResp.Proxies))

	var wg sync.WaitGroup

	for _, proxy := range blockedResp.Proxies {
		// Check if context is cancelled before starting new goroutine
		select {
		case <-ctx.Done():
			log.Info("Context cancelled, waiting for remaining goroutines to complete")
			wg.Wait()
			return ctx.Err()
		default:
		}

		// Acquire semaphore permit
		if err := pc.semaphore.Acquire(ctx, 1); err != nil {
			log.Info("Context cancelled while acquiring semaphore, waiting for remaining goroutines to complete")
			wg.Wait()
			return ctx.Err()
		}

		wg.Add(1)
		go func(proxy yarun.ProxyResponse) {
			defer wg.Done()
			defer pc.semaphore.Release(1)

			if err := pc.checkSingleProxy(ctx, proxy); err != nil {
				log.Error("Error checking proxy", "id", proxy.ID, "port", proxy.Port, "error", err)
			}
		}(proxy)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	return nil
}

// checkSingleProxy checks a single proxy for IP changes and accessibility
func (pc *ProxyChecker) checkSingleProxy(ctx context.Context, proxy yarun.ProxyResponse) error {
	log.Info("Checking proxy", "id", proxy.ID, "port", proxy.Port, "current_ip", proxy.IP)

	// Create proxy client
	proxyURL := fmt.Sprintf("http://%s:%s@gw.dataimpulse.com:%d", *pc.proxyUsername, *pc.proxyPassword, proxy.Port)

	// log.Info("proxy url", "url", proxyURL)

	proxyParsed, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyParsed),
	}

	proxyClient := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	// Check current IP
	currentIP, err := pc.getCurrentIP(ctx, proxyClient)
	if err != nil {
		log.Error("Failed to get current IP for proxy", "id", proxy.ID, "port", proxy.Port, "error", err)
		// If we can't get IP, assume it's still blocked and update last check
		return pc.updateProxyLastCheck(ctx, proxy.ID, proxy.IP)
	}

	// If IP has changed, unblock the proxy
	if currentIP != proxy.IP {
		log.Info("Proxy IP changed, unblocking", "id", proxy.ID, "port", proxy.Port, "old_ip", proxy.IP, "new_ip", currentIP)
		_, err := pc.yarunAPI.UnblockProxy(ctx, proxy.ID, currentIP, false)
		if err != nil {
			return fmt.Errorf("failed to unblock proxy: %w", err)
		}
		log.Info("Successfully unblocked proxy with new IP", "id", proxy.ID, "port", proxy.Port, "new_ip", currentIP)
		return nil
	}

	// IP is the same, check if the proxy is accessible
	isBlocked := pc.checkProxyBlocked(ctx, proxyClient)
	if isBlocked {
		log.Info("Proxy is still blocked, updating last check", "id", proxy.ID, "port", proxy.Port)
		return pc.updateProxyLastCheck(ctx, proxy.ID, proxy.IP)
	}

	// Proxy is accessible, unblock it
	log.Info("Proxy is now accessible, unblocking", "id", proxy.ID, "port", proxy.Port)
	_, err = pc.yarunAPI.UnblockProxy(ctx, proxy.ID, proxy.IP, false)
	if err != nil {
		return fmt.Errorf("failed to unblock proxy: %w", err)
	}
	log.Info("Successfully unblocked accessible proxy", "id", proxy.ID, "port", proxy.Port)
	return nil
}

// getCurrentIP gets the current IP of the proxy
func (pc *ProxyChecker) getCurrentIP(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.ipify.org/?format=json", nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ipify API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var ipResp IPifyResponse
	if err := json.Unmarshal(body, &ipResp); err != nil {
		return "", err
	}

	return ipResp.IP, nil
}

// checkProxyBlocked checks if the proxy is blocked by testing URLs
func (pc *ProxyChecker) checkProxyBlocked(ctx context.Context, client *http.Client) bool {
	for _, testURL := range pc.testURLs {
		if pc.isURLBlocked(ctx, client, testURL) {
			return true
		}
	}
	return false
}

// isURLBlocked checks if a specific URL is blocked
func (pc *ProxyChecker) isURLBlocked(ctx context.Context, client *http.Client, testURL string) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		log.Error("Failed to create request", "url", testURL, "error", err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error("Failed to fetch URL", "url", testURL, "error", err)
		return true // Assume blocked if we can't reach it
	}
	defer resp.Body.Close()

	// Check if status is not 200
	if resp.StatusCode != http.StatusOK {
		log.Warn("URL returned non-200 status, considering blocked", "url", testURL, "status", resp.StatusCode)
		return true
	}

	// Read response body to check for errorOccurPath
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("Failed to read response body", "url", testURL, "error", err)
		return false
	}

	bodyStr := string(body)
	if strings.Contains(bodyStr, "errorOccurPath") {
		log.Warn("URL contains errorOccurPath, blocked", "url", testURL)
		return true
	}

	return false
}

// updateProxyLastCheck updates the proxy's last check time while keeping it blocked
func (pc *ProxyChecker) updateProxyLastCheck(ctx context.Context, proxyID, ip string) error {
	_, err := pc.yarunAPI.UnblockProxy(ctx, proxyID, ip, true)
	if err != nil {
		return fmt.Errorf("failed to update proxy last check: %w", err)
	}
	return nil
}
