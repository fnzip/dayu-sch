package proxycheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"dayusch/internal/pkg/api/yarun"

	"github.com/charmbracelet/log"
)

// ProxyChecker handles proxy checking operations
type ProxyChecker struct {
	yarunAPI      *yarun.YarunApi
	limit         int
	testURLs      []string
	httpClient    *http.Client
	proxyUsername *string
	proxyPassword *string
}

// IPifyResponse represents the response from ipify API
type IPifyResponse struct {
	IP string `json:"ip"`
}

// NewProxyChecker creates a new proxy checker instance
func NewProxyChecker(yarunURL, yarunToken string, limit int) *ProxyChecker {
	proxyUsername := os.Getenv("PROXY_USERNAME")
	proxyPassword := os.Getenv("PROXY_PASSWORD")

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

	log.Printf("Found %d blocked proxies to check", len(blockedResp.Proxies))

	for i, proxy := range blockedResp.Proxies {
		// Check if context is cancelled before processing each proxy
		select {
		case <-ctx.Done():
			log.Printf("Context cancelled, stopping after checking %d/%d proxies", i, len(blockedResp.Proxies))
			return ctx.Err()
		default:
		}

		if err := pc.checkSingleProxy(ctx, proxy); err != nil {
			log.Printf("Error checking proxy %s:%d - %v", proxy.ID, proxy.Port, err)
		}

		// Add small delay between proxy checks to avoid overwhelming the system
		// But make it interruptible
		select {
		case <-ctx.Done():
			log.Printf("Context cancelled during delay, stopping after checking %d/%d proxies", i+1, len(blockedResp.Proxies))
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	return nil
}

// checkSingleProxy checks a single proxy for IP changes and accessibility
func (pc *ProxyChecker) checkSingleProxy(ctx context.Context, proxy yarun.ProxyResponse) error {
	log.Printf("Checking proxy %s:%d (current IP: %s)", proxy.ID, proxy.Port, proxy.IP)

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
		log.Printf("Failed to get current IP for proxy %s:%d - %v", proxy.ID, proxy.Port, err)
		// If we can't get IP, assume it's still blocked and update last check
		return pc.updateProxyLastCheck(ctx, proxy.ID, proxy.IP)
	}

	// If IP has changed, unblock the proxy
	if currentIP != proxy.IP {
		log.Printf("Proxy %s:%d IP changed from %s to %s - unblocking", proxy.ID, proxy.Port, proxy.IP, currentIP)
		_, err := pc.yarunAPI.UnblockProxy(ctx, proxy.ID, currentIP, false)
		if err != nil {
			return fmt.Errorf("failed to unblock proxy: %w", err)
		}
		log.Printf("Successfully unblocked proxy %s:%d with new IP %s", proxy.ID, proxy.Port, currentIP)
		return nil
	}

	// IP is the same, check if the proxy is accessible
	isBlocked := pc.checkProxyBlocked(ctx, proxyClient)
	if isBlocked {
		log.Printf("Proxy %s:%d is still blocked, updating last check", proxy.ID, proxy.Port)
		return pc.updateProxyLastCheck(ctx, proxy.ID, proxy.IP)
	}

	// Proxy is accessible, unblock it
	log.Printf("Proxy %s:%d is now accessible - unblocking", proxy.ID, proxy.Port)
	_, err = pc.yarunAPI.UnblockProxy(ctx, proxy.ID, proxy.IP, false)
	if err != nil {
		return fmt.Errorf("failed to unblock proxy: %w", err)
	}
	log.Printf("Successfully unblocked accessible proxy %s:%d", proxy.ID, proxy.Port)
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
		log.Printf("Failed to create request for %s: %v", testURL, err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to fetch %s: %v", testURL, err)
		return true // Assume blocked if we can't reach it
	}
	defer resp.Body.Close()

	// Check if status is not 200
	if resp.StatusCode != http.StatusOK {
		log.Printf("URL %s returned status %d - considering blocked", testURL, resp.StatusCode)
		return true
	}

	// Read response body to check for errorOccurPath
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body for %s: %v", testURL, err)
		return false
	}

	bodyStr := string(body)
	if strings.Contains(bodyStr, "errorOccurPath") {
		log.Printf("URL %s contains 'errorOccurPath' - blocked", testURL)
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
