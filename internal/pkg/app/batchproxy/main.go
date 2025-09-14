package batchproxy

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/goccy/go-yaml"
	"golang.org/x/sync/semaphore"

	cfbatch "dayusch/internal/pkg/api/cfbatch/v2"
	"dayusch/internal/pkg/api/yarun"
)

type Config struct {
	BaseURL       string `yaml:"base_url"`
	Token         string `yaml:"token"`
	ProxyUsername string `yaml:"proxy_username"`
	ProxyPassword string `yaml:"proxy_password"`
	YarunBaseURL  string `yaml:"yarun_base_url"`
	YarunToken    string `yaml:"yarun_token"`
}

func Run(maxConcurrent, batchLimit, delay uint, inputFile string) {
	var config Config

	if inputFile != "" {
		// Read and parse YAML config file
		data, err := os.ReadFile(inputFile)
		if err != nil {
			log.Fatal("Failed to read config file", "file", inputFile, "error", err)
		}

		if err := yaml.Unmarshal(data, &config); err != nil {
			log.Fatal("Failed to parse YAML config", "file", inputFile, "error", err)
		}

		log.Info("Loaded config from file", "file", inputFile)
	} else {
		// Read environment variables (fallback)
		config.BaseURL = os.Getenv("CFBATCH_URL")
		config.Token = os.Getenv("CFBATCH_TOKEN")
		config.ProxyUsername = os.Getenv("PROXY_USERNAME")
		config.ProxyPassword = os.Getenv("PROXY_PASSWORD")
		config.YarunBaseURL = os.Getenv("YARUN_BASE_URL")
		config.YarunToken = os.Getenv("YARUN_TOKEN")

		log.Info("Loaded config from environment variables")
	}

	if config.BaseURL == "" || config.Token == "" || config.ProxyUsername == "" || config.ProxyPassword == "" || config.YarunBaseURL == "" || config.YarunToken == "" {
		log.Fatal("Missing required configuration: base_url, token, proxy_username, proxy_password, yarun_base_url, yarun_token")
	}

	log.Info("Starting batchproxy",
		"baseURL", config.BaseURL,
		"yarunBaseURL", config.YarunBaseURL,
		"maxConcurrent", maxConcurrent,
		"batchLimit", batchLimit,
		"delay", delay,
	)

	// Create parent CFBatchApi
	parentApi := cfbatch.NewCFBatchApi(config.BaseURL, config.Token)

	// Create yarun API client
	yarunClient := yarun.NewYarunApi(config.YarunBaseURL, config.YarunToken)

	log.Info("Created parent CFBatchApi and yarun client instances")

	for {
		log.Info("Starting new batch round")

		// Get available proxies from yarun
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		proxiesResp, err := yarunClient.GetProxies(ctx, int(maxConcurrent))
		cancel()

		if err != nil {
			log.Error("Failed to get proxies from yarun", "error", err)
			time.Sleep(time.Duration(delay) * time.Second)
			continue
		}

		if len(proxiesResp.Proxies) == 0 {
			log.Warn("No available proxies returned from yarun")
			time.Sleep(time.Duration(delay) * time.Second)
			continue
		}

		log.Info("Got proxies from yarun", "count", len(proxiesResp.Proxies))

		// Create semaphore for controlling concurrency
		sem := semaphore.NewWeighted(int64(maxConcurrent))
		var wg sync.WaitGroup

		// Create concurrent workers using available proxies
		for i, proxy := range proxiesResp.Proxies {
			if i >= int(maxConcurrent) {
				break // Don't exceed maxConcurrent
			}

			wg.Add(1)
			go func(workerID int, proxyPort int) {
				defer wg.Done()

				// Acquire semaphore
				if err := sem.Acquire(context.Background(), 1); err != nil {
					log.Error("Failed to acquire semaphore", "workerID", workerID, "error", err)
					return
				}
				defer sem.Release(1)

				log.Info("Worker started", "workerID", workerID, "assignedPort", proxyPort)

				// Send batch request
				ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()

				api := parentApi.Clone()

				// Set user agent first (round-robin)
				userAgent := GetNextUserAgent()
				api.SetUserAgent(userAgent)

				// Then set proxy URL
				proxyURL := fmt.Sprintf("http://%s:%s@gw.dataimpulse.com:%d", config.ProxyUsername, config.ProxyPassword, proxyPort)
				api.SetProxyURL(proxyURL)

				log.Info("Proxy and User-Agent configured", "workerID", workerID, "port", proxyPort, "userAgent", userAgent[:16]+"...")

				responses, err := api.SendBatch(ctx, int(batchLimit))
				shouldBlockProxy := false

				if err != nil {
					log.Error("SendBatch failed", "workerID", workerID, "port", proxyPort, "error", err)
					shouldBlockProxy = true
				} else {
					log.Info("SendBatch completed successfully",
						"workerID", workerID,
						"limit", batchLimit,
						"responseCount", len(responses))

					// Analyze response status to determine if proxy should be blocked
					var failedCount int
					totalCount := len(responses)

					// Print each response and count failures
					for i, response := range responses {
						if !response.Status {
							failedCount++
						}

						if response.Result != nil {
							log.Info("Batch response",
								"workerID", workerID,
								"i", i,
								"a", response.App,
								"u", response.Username,
								"s", response.Status,
								"b", response.Result.Balance,
								"c", response.Result.Coin)
						} else {
							log.Info("Batch response",
								"workerID", workerID,
								"i", i,
								"a", response.App,
								"u", response.Username,
								"s", response.Status,
								"b", "nil",
								"c", "nil")
						}
					}

					// Check if failure rate is >= 50%
					if totalCount > 0 {
						failureRate := float64(failedCount) / float64(totalCount)
						log.Info("Batch response analysis",
							"workerID", workerID,
							"totalResponses", totalCount,
							"failedResponses", failedCount,
							"failureRate", fmt.Sprintf("%.2f%%", failureRate*100))

						if failureRate >= 0.5 {
							shouldBlockProxy = true
							log.Warn("High failure rate detected, will block proxy",
								"workerID", workerID,
								"port", proxyPort,
								"failureRate", fmt.Sprintf("%.2f%%", failureRate*100))
						}
					}
				}

				// Block proxy if needed (either due to API error or high failure rate)
				if shouldBlockProxy {
					blockCtx, blockCancel := context.WithTimeout(context.Background(), 30*time.Second)
					proxyID := fmt.Sprintf("port_%d", proxyPort) // Assuming proxy ID format
					_, blockErr := yarunClient.BlockProxy(blockCtx, proxyID)
					blockCancel()

					if blockErr != nil {
						log.Error("Failed to block proxy", "workerID", workerID, "port", proxyPort, "error", blockErr)
					} else {
						log.Info("Proxy blocked", "workerID", workerID, "port", proxyPort, "reason", func() string {
							if err != nil {
								return "API error"
							}
							return "high failure rate (>=50%)"
						}())
					}
				}
			}(i, proxy.Port)
		}

		// Wait for all workers to complete
		log.Info("Waiting for all workers to complete")
		wg.Wait()
		log.Info("All workers completed, starting next round")

		// Optional: add a small delay between rounds
		time.Sleep(time.Duration(delay) * time.Second)
	}
}
