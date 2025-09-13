package main

import (
	"dayusch/internal/pkg/app/batchproxy"
	"flag"
)

func main() {
	// Parse command line flags
	var (
		maxConcurrent = flag.Uint("concurrent", 10, "Number of concurrent workers")
		concurrentC   = flag.Uint("c", 10, "Number of concurrent workers (shorthand)")
		batchLimit    = flag.Uint("batch", 10, "Batch limit")
		batchB        = flag.Uint("b", 10, "Batch limit (shorthand)")
		delay         = flag.Uint("delay", 0, "Delay between rounds in seconds")
		delayD        = flag.Uint("d", 0, "Delay between rounds in seconds (shorthand)")
		inputFile     = flag.String("i", "", "Input YAML config file")
	)
	flag.Parse()

	// Use shorthand flags if they were explicitly set
	finalConcurrent := *maxConcurrent
	if *concurrentC != 10 {
		finalConcurrent = *concurrentC
	}

	finalBatch := *batchLimit
	if *batchB != 10 {
		finalBatch = *batchB
	}

	finalDelay := *delay
	if *delayD != 1 {
		finalDelay = *delayD
	}

	batchproxy.Run(finalConcurrent, finalBatch, finalDelay, *inputFile)
}
