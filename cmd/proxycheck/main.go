package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dayusch/internal/pkg/app/proxycheck"

	"github.com/charmbracelet/log"
)

func main() {
	var (
		yarunURL      = flag.String("yarun-url", "", "Yarun API base URL")
		yarunToken    = flag.String("yarun-token", "", "Yarun API token")
		proxyUsername = flag.String("proxy-username", "", "Proxy username")
		proxyPassword = flag.String("proxy-password", "", "Proxy password")
		interval      = flag.Duration("interval", 5*time.Second, "Check interval")
		limit         = flag.Int("limit", 32, "Limit of blocked proxies to check")
	)
	flag.Parse()

	if *yarunURL == "" || *yarunToken == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -yarun-url <url> -yarun-token <token> [options]\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Create proxy checker
	checker := proxycheck.NewProxyChecker(*yarunURL, *yarunToken, *proxyUsername, *proxyPassword, *limit)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Info("Received shutdown signal, stopping...")
		cancel()
	}()

	log.Info("Starting proxy checker", "interval", *interval, "limit", *limit)
	log.Info("Yarun API configured", "url", *yarunURL)

	// Start the checking loop
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	// Run initial check
	if err := checker.CheckProxies(ctx); err != nil {
		log.Error("Initial check failed", "error", err)
	}

	// Continue checking at intervals
	for {
		select {
		case <-ctx.Done():
			log.Info("Shutting down proxy checker")
			return
		case <-ticker.C:
			if err := checker.CheckProxies(ctx); err != nil {
				log.Error("Check failed", "error", err)
			}
		}
	}
}
