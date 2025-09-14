package batchproxy

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/goccy/go-yaml"
	"golang.org/x/sync/semaphore"

	cfbatch "dayusch/internal/pkg/api/cfbatch/v2"
)

const (
	proxyPortStart = 10000 // Start of port range
	proxyPortEnd   = 19998 // End of port range
)

type Config struct {
	BaseURL       string `yaml:"base_url"`
	Token         string `yaml:"token"`
	ProxyUsername string `yaml:"proxy_username"`
	ProxyPassword string `yaml:"proxy_password"`
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

		log.Info("Loaded config from environment variables")
	}

	if config.BaseURL == "" || config.Token == "" || config.ProxyUsername == "" || config.ProxyPassword == "" {
		log.Fatal("Missing required configuration: base_url, token, proxy_username, proxy_password")
	}

	log.Info("Starting batchproxy",
		"baseURL", config.BaseURL,
		"maxConcurrent", maxConcurrent,
		"batchLimit", batchLimit,
		"delay", delay,
		"proxyPortRange", fmt.Sprintf("%d-%d", proxyPortStart, proxyPortEnd),
	)

	// Create parent CFBatchApi
	parentApi := cfbatch.NewCFBatchApi(config.BaseURL, config.Token)

	// Initialize random starting port
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	currentPort := proxyPortStart + r.Intn(proxyPortEnd-proxyPortStart+1)

	log.Info("Created parent CFBatchApi instance", "startingPort", currentPort)

	for {
		log.Info("Starting new batch round", "startingPort", currentPort)

		// Create semaphore for controlling concurrency
		sem := semaphore.NewWeighted(int64(maxConcurrent))
		var wg sync.WaitGroup

		// Create concurrent workers
		for i := 0; i < int(maxConcurrent); i++ {
			wg.Add(1)
			go func(workerID int, port int) {
				defer wg.Done()

				// Acquire semaphore
				if err := sem.Acquire(context.Background(), 1); err != nil {
					log.Error("Failed to acquire semaphore", "workerID", workerID, "error", err)
					return
				}
				defer sem.Release(1)

				log.Info("Worker started", "workerID", workerID, "assignedPort", port)

				// Send batch request
				ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()

				api := parentApi.Clone()

				// Set user agent first (round-robin)
				userAgent := GetNextUserAgent()
				api.SetUserAgent(userAgent)

				// Then set proxy URL
				proxyURL := fmt.Sprintf("http://%s:%s@gw.dataimpulse.com:%d", config.ProxyUsername, config.ProxyPassword, port)
				api.SetProxyURL(proxyURL)

				log.Info("Proxy and User-Agent configured", "workerID", workerID, "port", port, "userAgent", userAgent[:16]+"...")

				responses, err := api.SendBatch(ctx, int(batchLimit))
				if err != nil {
					log.Error("SendBatch failed", "workerID", workerID, "port", port, "error", err)
				} else {
					log.Info("SendBatch completed successfully",
						"workerID", workerID,
						"limit", batchLimit,
						"responseCount", len(responses))

					// Print each response
					for i, response := range responses {
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
				}
			}(i, currentPort)

			// Move to next port in round robin
			currentPort++
			if currentPort > proxyPortEnd {
				currentPort = proxyPortStart
			}
		}

		// Wait for all workers to complete
		log.Info("Waiting for all workers to complete")
		wg.Wait()
		log.Info("All workers completed, starting next round")

		// Optional: add a small delay between rounds
		time.Sleep(time.Duration(delay) * time.Second)
	}
}
