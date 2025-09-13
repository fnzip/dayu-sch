package batchproxy

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"golang.org/x/sync/semaphore"

	cfbatch_v2 "dayusch/internal/pkg/api/cfbatch/v2"
)

const (
	proxyPort = 823 // Sticky port with automatic rotation
)

func Run(maxConcurrent, batchLimit, delay uint) {
	// Read environment variables
	baseURL := os.Getenv("CFBATCH_URL")
	token := os.Getenv("CFBATCH_TOKEN")
	proxyUsername := os.Getenv("PROXY_USERNAME")
	proxyPassword := os.Getenv("PROXY_PASSWORD")

	if baseURL == "" || token == "" || proxyUsername == "" || proxyPassword == "" {
		log.Fatal("Missing required environment variables: CFBATCH_URL, CFBATCH_TOKEN, PROXY_USERNAME, PROXY_PASSWORD")
	}

	log.Info("Starting batchproxy",
		"baseURL", baseURL,
		"maxConcurrent", maxConcurrent,
		"batchLimit", batchLimit,
		"delay", delay)

	// Create parent CFBatchApi
	api := cfbatch_v2.NewCFBatchApi(baseURL, token)
	// Construct proxy URL for dataimpulse with sticky port (auto-rotating)
	proxyURL := fmt.Sprintf("http://%s:%s@gw.dataimpulse.com:%d", proxyUsername, proxyPassword, proxyPort)
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

				log.Info("Worker started", "workerID", workerID, "proxyPort", proxyPort)

				// Send batch request
				ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()

				err := api.SendBatch(ctx, int(batchLimit))
				if err != nil {
					log.Error("SendBatch failed", "workerID", workerID, "proxyPort", proxyPort, "error", err)
				} else {
					log.Info("SendBatch completed successfully", "workerID", workerID, "proxyPort", proxyPort, "limit", batchLimit)
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
