package batchproxy

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/goccy/go-yaml"
	"golang.org/x/sync/semaphore"

	cfbatch "dayusch/internal/pkg/api/cfbatch/v2"
	"dayusch/internal/pkg/api/yarun"
	"dayusch/internal/pkg/helper"
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

	// Create a root context that will be cancelled on shutdown
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info("Received shutdown signal, stopping gracefully...")
		rootCancel() // This will cancel all derived contexts
	}()

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
		// Check for shutdown signal
		select {
		case <-rootCtx.Done():
			log.Info("Shutdown requested, stopping main loop")
			return
		default:
		}

		log.Info("Starting new batch round")

		// Get available proxies from yarun
		ctx, cancel := context.WithTimeout(rootCtx, 30*time.Second)
		proxiesResp, err := yarunClient.GetProxies(ctx, int(maxConcurrent))
		cancel()

		if err != nil {
			if ctx.Err() == context.Canceled {
				log.Info("Proxy request cancelled due to shutdown")
				return
			}
			log.Error("Failed to get proxies from yarun", "error", err)

			// Check for shutdown before sleeping
			select {
			case <-rootCtx.Done():
				log.Info("Shutdown requested during delay")
				return
			case <-time.After(time.Duration(delay) * time.Second):
			}
			continue
		}

		if len(proxiesResp.Proxies) == 0 {
			log.Warn("No available proxies returned from yarun")

			// Check for shutdown before sleeping
			select {
			case <-rootCtx.Done():
				log.Info("Shutdown requested during delay")
				return
			case <-time.After(time.Duration(delay) * time.Second):
			}
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
			go func(workerID int, proxy yarun.ProxyResponse) {
				defer func() {
					log.Info("Worker finished", "workerID", workerID, "assignedPort", proxy.Port)
					wg.Done()
				}()

				// Add timeout for entire worker
				workerCtx, workerCancel := context.WithTimeout(rootCtx, 10*time.Minute)
				defer workerCancel()

				// Check for early cancellation
				select {
				case <-workerCtx.Done():
					log.Warn("Worker cancelled before starting", "workerID", workerID)
					return
				default:
				}

				// Acquire semaphore
				if err := sem.Acquire(context.Background(), 1); err != nil {
					log.Error("Failed to acquire semaphore", "workerID", workerID, "error", err)
					return
				}
				defer sem.Release(1)

				log.Info("Worker started", "workerID", workerID, "assignedPort", proxy.Port)

				// Send batch request with worker context
				ctx, cancel := context.WithTimeout(workerCtx, 60*time.Second)
				defer cancel()

				api := parentApi.Clone()

				// Set user agent first (round-robin)
				userAgent := helper.GetNextUserAgent()
				api.SetUserAgent(userAgent)

				// Then set proxy URL
				proxyURL := fmt.Sprintf("http://%s:%s@gw.dataimpulse.com:%d", config.ProxyUsername, config.ProxyPassword, proxy.Port)
				api.SetProxyURL(proxyURL)

				log.Info("Proxy and User-Agent configured", "workerID", workerID, "port", proxy.Port, "userAgent", userAgent[:16]+"...")

				responses, err := api.SendBatch(ctx, int(batchLimit))
				shouldBlockProxy := false

				if err != nil {
					if ctx.Err() == context.Canceled {
						log.Info("Batch request cancelled due to shutdown", "workerID", workerID)
						return
					}
					if ctx.Err() == context.DeadlineExceeded {
						log.Warn("Batch request timeout", "workerID", workerID, "port", proxy.Port, "timeout", "60s")
					}
					log.Error("SendBatch failed", "workerID", workerID, "port", proxy.Port, "error", err)
					shouldBlockProxy = true
				} else {
					log.Info("SendBatch completed successfully",
						"workerID", workerID,
						"limit", batchLimit,
						"responseCount", len(responses))

					// Analyze response status to determine if proxy should be blocked
					var failedCount int32
					totalCount := len(responses)

					// Process each response concurrently using goroutines
					var responseWg sync.WaitGroup
					responseSem := semaphore.NewWeighted(int64(len(responses))) // Allow all responses to run concurrently

					for i, response := range responses {
						responseWg.Add(1)
						go func(idx int, resp cfbatch.BatchResponse) {
							defer responseWg.Done()

							// Add timeout for entire response processing
							responseCtx, responseCancel := context.WithTimeout(workerCtx, 2*time.Minute)
							defer responseCancel()

							// Check for shutdown signal
							select {
							case <-responseCtx.Done():
								if responseCtx.Err() == context.DeadlineExceeded {
									log.Warn("Response processing timeout", "responseIndex", idx, "workerID", workerID)
								}
								return
							default:
							}

							// Acquire semaphore for this response processing
							if err := responseSem.Acquire(responseCtx, 1); err != nil {
								if err == context.Canceled || err == context.DeadlineExceeded {
									log.Info("Response processing cancelled or timed out")
									return
								}
								log.Error("Failed to acquire response semaphore", "error", err)
								return
							}
							defer responseSem.Release(1)

							if !resp.Status {
								atomic.AddInt32(&failedCount, 1)
							}

							if resp.Result != nil {
								log.Info("Batch response",
									"workerID", workerID,
									"i", idx,
									"a", resp.App,
									"u", resp.Username,
									"s", resp.Status,
									"b", resp.Result.Balance,
									"c", resp.Result.Coin)
							} else {
								log.Info("Batch response",
									"workerID", workerID,
									"i", idx,
									"a", resp.App,
									"u", resp.Username,
									"s", resp.Status,
									"b", "nil",
									"c", "nil")
							}
						}(i, response)
					}

					// Wait for all response processing to complete
					log.Info("Waiting for all response processing to complete", "totalResponses", totalCount)
					responseWg.Wait()
					log.Info("All response processing completed")

					// Check if failure rate is >= 50%
					if totalCount > 0 {
						finalFailedCount := atomic.LoadInt32(&failedCount)
						failureRate := float64(finalFailedCount) / float64(totalCount)
						log.Info("Batch response analysis",
							"workerID", workerID,
							"totalResponses", totalCount,
							"failedResponses", finalFailedCount,
							"failureRate", fmt.Sprintf("%.2f%%", failureRate*100))

						if failureRate >= 0.5 {
							shouldBlockProxy = true
							log.Warn("High failure rate detected, will block proxy",
								"workerID", workerID,
								"port", proxy.Port,
								"failureRate", fmt.Sprintf("%.2f%%", failureRate*100))
						}
					}
				}

				// Block proxy if needed (either due to API error or high failure rate)
				if shouldBlockProxy {
					blockCtx, blockCancel := context.WithTimeout(workerCtx, 30*time.Second)
					_, blockErr := yarunClient.BlockProxy(blockCtx, proxy.ID)
					blockCancel()

					if blockErr != nil {
						if blockCtx.Err() == context.Canceled || blockCtx.Err() == context.DeadlineExceeded {
							log.Info("Block proxy cancelled or timed out", "workerID", workerID)
							return
						}
						log.Error("Failed to block proxy", "workerID", workerID, "port", proxy.Port, "error", blockErr)
					} else {
						log.Info("Proxy blocked", "workerID", workerID, "port", proxy.Port, "reason", func() string {
							if err != nil {
								return "API error"
							}
							return "high failure rate (>=50%)"
						}())
					}
				}
			}(i, proxy)
		}

		// Wait for all workers to complete with timeout monitoring
		workersDone := make(chan struct{})
		go func() {
			wg.Wait()
			close(workersDone)
		}()

		log.Info("Waiting for all workers to complete")

		// Monitor workers with periodic logging
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-workersDone:
				log.Info("All workers completed, starting next round")
				goto nextRound
			case <-ticker.C:
				log.Info("Still waiting for workers to complete...")
			case <-rootCtx.Done():
				log.Info("Shutdown requested while waiting for workers")
				return
			}
		}

	nextRound:

		// Check for shutdown signal before delay
		select {
		case <-rootCtx.Done():
			log.Info("Shutdown requested, exiting main loop")
			return
		case <-time.After(time.Duration(delay) * time.Second):
		}
	}
}
