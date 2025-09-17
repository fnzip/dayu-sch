package batchproxyplay

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
	"dayusch/internal/pkg/api/pragmatic"
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
			go func(workerID int, proxy yarun.ProxyResponse) {
				defer wg.Done()

				// Acquire semaphore
				if err := sem.Acquire(context.Background(), 1); err != nil {
					log.Error("Failed to acquire semaphore", "workerID", workerID, "error", err)
					return
				}
				defer sem.Release(1)

				log.Info("Worker started", "workerID", workerID, "assignedPort", proxy.Port)

				// Send batch request
				ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()

				api := parentApi.Clone()

				// Set user agent first (round-robin)
				userAgent := helper.GetNextUserAgent()
				api.SetUserAgent(userAgent)

				// Then set proxy URL
				proxyURL := fmt.Sprintf("http://%s:%s@gw.dataimpulse.com:%d", config.ProxyUsername, config.ProxyPassword, proxy.Port)
				api.SetProxyURL(proxyURL)

				responses, err := api.GetBatchLink(ctx, int(batchLimit))
				shouldBlockProxy := false

				if err != nil {
					log.Error("SendBatch failed", "workerID", workerID, "port", proxy.Port, "error", err)
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
					for _, response := range responses {
						if !response.Status {
							failedCount++
						}

						if response.Link != nil {
							// state
							var index *int
							var counter *int

							pp := pragmatic.NewPragmaticPlay(ctx, *response.Link, userAgent)

							sessionData, err := pp.LoadSession()
							if err != nil {
								log.Error("error on load game", "error", err)
								return
							}

							if sessionData == nil {
								log.Error("error on load game: sessionData is nil")
								return
							}

							initResData, err := pp.DoInit(sessionData.MGCKey, response.GameSymbol)
							if err != nil {
								log.Error("error on init game", "error", err)
								return
							}

							tmpIndex := initResData.Index + 1
							index = &tmpIndex
							tmpCounter := initResData.Counter + 1
							counter = &tmpCounter

							log.Info("user info", "balance", initResData.Balance, "total_win", initResData.TotalWin, "next_action", initResData.NextAction)

							// loop for spin
							for {
								// Check balance threshold
								if initResData.NextAction == "s" && (initResData.Balance <= 500.0 || initResData.Balance >= 100_000.0) {
									log.Info("threshold reached, stopping loop", "balance", initResData.Balance)

									if initResData.Balance >= 100_000.0 {
										log.Info("JACKPOT", "balance", initResData.Balance)
									}

									// tmx.yarunApi.UpdateUserBalance(user.ID, initResData.Balance, homeData.Data.AmountInfo.UsableCurrency)
									_, err := yarunClient.UpdateUserBalance(ctx, response.ID, initResData.Balance, response.Coin)
									if err != nil {
										log.Error("error on update user balance", "error", err)

										return
									}

									break
								}

								// Progressive coin logic
								balance := initResData.Balance
								amount := 400.0
								if balance > 10000 && balance <= 30000 {
									amount = 600.0
								} else if balance > 30000 && balance <= 50000 {
									amount = 800.0
								} else if balance > 50000 && balance <= 100000 {
									amount = 1000.0
								}

								coinValue := int(amount / 20.0)
								coin := &coinValue

								if initResData.NextAction == "s" {
									respData, err := pp.DoSpin(sessionData.MGCKey, response.GameSymbol, *coin, *index, *counter, "aq")
									if err != nil {
										log.Error("error on spin game", "error", err)

										return
									}

									tmpIndex = respData.Index + 1
									index = &tmpIndex
									tmpCounter = respData.Counter + 1
									counter = &tmpCounter

									log.Info("spin action info", "balance", respData.Balance, "total_win", respData.TotalWin, "new_index", *index, "new_counter", *counter, "next_action", respData.NextAction, "coin", *coin, "amount", amount)
									initResData = respData // update state for next action
								}

								if initResData.NextAction == "c" {
									respData, err := pp.DoCollect(sessionData.MGCKey, response.GameSymbol, *index, *counter)
									if err != nil {
										log.Error("error on collect", "error", err)

										return
									}

									tmpIndex = respData.Index + 1
									index = &tmpIndex
									tmpCounter = respData.Counter + 1
									counter = &tmpCounter

									log.Info("collect action info", "balance", respData.Balance, "total_win", respData.TotalWin, "new_index", *index, "new_counter", *counter, "next_action", respData.NextAction)
									initResData = respData // update state for next action
								}

								time.Sleep(32 * time.Millisecond)
							}
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
								"port", proxy.Port,
								"failureRate", fmt.Sprintf("%.2f%%", failureRate*100))
						}
					}
				}

				// Block proxy if needed (either due to API error or high failure rate)
				if shouldBlockProxy {
					blockCtx, blockCancel := context.WithTimeout(context.Background(), 30*time.Second)
					_, blockErr := yarunClient.BlockProxy(blockCtx, proxy.ID)
					blockCancel()

					if blockErr != nil {
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

		// Wait for all workers to complete
		log.Info("Waiting for all workers to complete")
		wg.Wait()
		log.Info("All workers completed, starting next round")

		// Optional: add a small delay between rounds
		time.Sleep(time.Duration(delay) * time.Second)
	}
}
