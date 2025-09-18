package main

import (
	"context"
	"dayusch/internal/pkg/app/dupdel"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown on Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Info("Received interrupt signal, shutting down...")
		cancel()
	}()

	dupDel := dupdel.NewDupDel(ctx)
	dupDel.Run()
}
