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

	cfbatch_v2 "dayusch/internal/pkg/api/cfbatch/v2"
)

const (
	proxyPort = 823 // Sticky port with automatic rotation
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
		"proxyPort", proxyPort,
	)

	// Create parent CFBatchApi
	api := cfbatch_v2.NewCFBatchApi(config.BaseURL, config.Token)
	// Construct proxy URL for dataimpulse with sticky port (auto-rotating)
	proxyURL := fmt.Sprintf("http://%s:%s@gw.dataimpulse.com:%d", config.ProxyUsername, config.ProxyPassword, proxyPort)
	api.SetProxyURL(proxyURL)

	log.Info("Created parent CFBatchApi instance")

	for {
		log.Info("Starting new batch round")

		// Create semaphore for controlling concurrency
		sem := semaphore.NewWeighted(int64(maxConcurrent))
		var wg sync.WaitGroup

		// Create concurrent workers
		for i := 0; i < int(maxConcurrent); i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				// Acquire semaphore
				if err := sem.Acquire(context.Background(), 1); err != nil {
					log.Error("Failed to acquire semaphore", "workerID", workerID, "error", err)
					return
				}
				defer sem.Release(1)

				log.Info("Worker started", "workerID", workerID)

				// Send batch request
				ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()

				responses, err := api.SendBatch(ctx, int(batchLimit))
				if err != nil {
					log.Error("SendBatch failed", "workerID", workerID, "error", err)
				} else {
					log.Info("SendBatch completed successfully",
						"workerID", workerID,
						"limit", batchLimit,
						"responseCount", len(responses))

					// Print each response
					for i, response := range responses {
						log.Info("Batch response",
							"workerID", workerID,
							"index", i,
							"app", response.App,
							"username", response.Username,
							"status", response.Status,
							"balance", response.Result.Balance,
							"coin", response.Result.Coin)
					}
				}
			}(i)
		}

		// Wait for all workers to complete
		log.Info("Waiting for all workers to complete")
		wg.Wait()
		log.Info("All workers completed, starting next round")

		// Optional: add a small delay between rounds
		time.Sleep(time.Duration(delay) * time.Second)
	}
}
