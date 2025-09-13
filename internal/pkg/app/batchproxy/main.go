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
	proxyPortStart = 10000
	proxyPortEnd   = 19998
	maxConcurrent  = 10
	batchLimit     = 16 // adjust as needed
)

func Run() {
	// Read environment variables
	baseURL := os.Getenv("CFBATCH_URL")
	token := os.Getenv("CFBATCH_TOKEN")
	proxyUsername := os.Getenv("PROXY_USERNAME")
	proxyPassword := os.Getenv("PROXY_PASSWORD")

	if baseURL == "" || token == "" || proxyUsername == "" || proxyPassword == "" {
		log.Fatal("Missing required environment variables: CFBATCH_URL, CFBATCH_TOKEN, PROXY_USERNAME, PROXY_PASSWORD")
	}

	log.Info("Starting batchproxy", "baseURL", baseURL)

	// Create parent CFBatchApi
	parentApi := cfbatch_v2.NewCFBatchApi(baseURL, token)
	log.Info("Created parent CFBatchApi instance")

	// Initialize round robin proxy port
	currentPort := proxyPortStart

	for {
		log.Info("Starting new batch round", "startingPort", currentPort)

		// Create semaphore for controlling concurrency
		sem := semaphore.NewWeighted(maxConcurrent)
		var wg sync.WaitGroup

		// Create 10 concurrent workers
		for i := 0; i < maxConcurrent; i++ {
			wg.Add(1)
			go func(workerID int, port int) {
				defer wg.Done()

				// Acquire semaphore
				if err := sem.Acquire(context.Background(), 1); err != nil {
					log.Error("Failed to acquire semaphore", "workerID", workerID, "error", err)
					return
				}
				defer sem.Release(1)

				// Clone the parent API
				api := parentApi.Clone()

				// Construct proxy URL for dataimpulse
				proxyURL := fmt.Sprintf("http://%s:%s@gw.dataimpulse.com:%d", proxyUsername, proxyPassword, port)
				api.SetProxyURL(proxyURL)

				log.Info("Worker started", "workerID", workerID, "proxyPort", port)

				// Send batch request
				ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				defer cancel()

				err := api.SendBatch(ctx, batchLimit)
				if err != nil {
					log.Error("SendBatch failed", "workerID", workerID, "proxyPort", port, "error", err)
				} else {
					log.Info("SendBatch completed successfully", "workerID", workerID, "proxyPort", port, "limit", batchLimit)
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
		// time.Sleep(1 * time.Second)
	}
}
