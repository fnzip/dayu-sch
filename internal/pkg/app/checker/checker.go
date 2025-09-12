package checker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// SiteInfo represents the structure of the response from /__dayu/siteInfo.html
type SiteInfo struct {
	OperateArea        string            `json:"operate_area"`
	TemplateCode       string            `json:"templateCode"`
	CustomerServiceUrl string            `json:"customerServiceUrl"`
	CfTurnstileSiteKey string            `json:"cf_turnstile_siteKey"`
	CfTurnstileSwitch  string            `json:"cf_turnstile_switch"`
	Timezone           string            `json:"timezone"`
	MaintenanceEndTime string            `json:"maintenanceEndTime"`
	SupportLanguages   map[string]string `json:"supportLanguages"`
	MainLanguage       string            `json:"mainLanguage"`
	IpaDownloadUrl     string            `json:"ipaDownloadUrl"`
}

// CheckResult represents the result of checking a domain
type CheckResult struct {
	Domain  string
	Success bool
	Error   string
}

// Checker handles the domain checking functionality
type Checker struct {
	httpClient *http.Client
	workers    int
}

// NewChecker creates a new checker instance
func NewChecker(workers int) *Checker {
	return &Checker{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		workers: workers,
	}
}

// ReadDomains reads domains from input file
func (c *Checker) ReadDomains(inputFile string) ([]string, error) {
	file, err := os.Open(inputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	var domains []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		domain := strings.TrimSpace(scanner.Text())
		if domain != "" {
			domains = append(domains, domain)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read input file: %w", err)
	}

	return domains, nil
}

// CheckDomain checks a single domain for the presence of operate_area field
func (c *Checker) CheckDomain(domain string) CheckResult {
	url := fmt.Sprintf("https://%s/__dayu/siteInfo.html", domain)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return CheckResult{
			Domain:  domain,
			Success: false,
			Error:   fmt.Sprintf("HTTP request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CheckResult{
			Domain:  domain,
			Success: false,
			Error:   fmt.Sprintf("HTTP status: %d", resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CheckResult{
			Domain:  domain,
			Success: false,
			Error:   fmt.Sprintf("Failed to read response body: %v", err),
		}
	}

	var siteInfo SiteInfo
	if err := json.Unmarshal(body, &siteInfo); err != nil {
		return CheckResult{
			Domain:  domain,
			Success: false,
			Error:   fmt.Sprintf("Failed to parse JSON: %v", err),
		}
	}

	// Check if operate_area field exists and is not empty
	if siteInfo.OperateArea == "" {
		return CheckResult{
			Domain:  domain,
			Success: false,
			Error:   "operate_area field is missing or empty",
		}
	}

	return CheckResult{
		Domain:  domain,
		Success: true,
		Error:   "",
	}
}

// CheckDomains checks multiple domains concurrently
func (c *Checker) CheckDomains(domains []string) []CheckResult {
	jobs := make(chan string, len(domains))
	results := make(chan CheckResult, len(domains))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < c.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for domain := range jobs {
				result := c.CheckDomain(domain)
				results <- result
			}
		}()
	}

	// Send jobs
	for _, domain := range domains {
		jobs <- domain
	}
	close(jobs)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var allResults []CheckResult
	for result := range results {
		allResults = append(allResults, result)
	}

	return allResults
}

// WriteSuccessfulDomains writes successful domains to output file
func (c *Checker) WriteSuccessfulDomains(results []CheckResult, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for _, result := range results {
		if result.Success {
			if _, err := writer.WriteString(result.Domain + "\n"); err != nil {
				return fmt.Errorf("failed to write to output file: %w", err)
			}
		}
	}

	return nil
}
