package main

import (
	"dayusch/internal/pkg/app/batchproxyplay"
	"flag"
)

func main() {
	// Parse command line flags
	var (
		concurrent = flag.Uint("c", 10, "Number of concurrent workers")
		batch      = flag.Uint("b", 10, "Batch limit")
		delay      = flag.Uint("d", 0, "Delay between rounds in seconds")
		inputFile  = flag.String("i", "", "Input YAML config file")
	)
	flag.Parse()

	batchproxyplay.Run(*concurrent, *batch, *delay, *inputFile)
}
