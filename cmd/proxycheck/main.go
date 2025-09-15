package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dayusch/internal/pkg/app/proxycheck"
)

func main() {
	var (
		yarunURL   = flag.String("yarun-url", "", "Yarun API base URL")
		yarunToken = flag.String("yarun-token", "", "Yarun API token")
		interval   = flag.Duration("interval", 5*time.Second, "Check interval")
		limit      = flag.Int("limit", 32, "Limit of blocked proxies to check")
	)
	flag.Parse()

	if *yarunURL == "" || *yarunToken == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -yarun-url <url> -yarun-token <token> [options]\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Create proxy checker
	checker := proxycheck.NewProxyChecker(*yarunURL, *yarunToken, *limit)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Received shutdown signal, stopping...")
		cancel()
	}()

	log.Printf("Starting proxy checker with interval %v, limit %d", *interval, *limit)
	log.Printf("Yarun API: %s", *yarunURL)

	// Start the checking loop
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	// Run initial check
	if err := checker.CheckProxies(ctx); err != nil {
		log.Printf("Initial check failed: %v", err)
	}

	// Continue checking at intervals
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down proxy checker")
			return
		case <-ticker.C:
			if err := checker.CheckProxies(ctx); err != nil {
				log.Printf("Check failed: %v", err)
			}
		}
	}
}
