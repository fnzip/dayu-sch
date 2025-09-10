package main

import (
	"context"
	"dayusch/internal/pkg/app/batch"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
)

func main() {
	ctxParent := context.Background()
	ctx, cancel := context.WithCancel(ctxParent)
	defer cancel()

	app := batch.NewBatchApp(ctx)

	go func() {
		// Wait for interrupt signal to gracefully shutdown the application
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs

		log.Info("shutting down...")
		cancel()
	}()

	app.Run()
}
