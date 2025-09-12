package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"dayusch/internal/pkg/app/checker"
)

func main() {
	var (
		inputFile  = flag.String("i", "", "Input file containing domains (one per line)")
		outputFile = flag.String("o", "", "Output file for successful domains")
		workers    = flag.Int("w", 10, "Number of concurrent workers")
		help       = flag.Bool("h", false, "Show help")
	)
	flag.Parse()

	if *help || *inputFile == "" || *outputFile == "" {
		showHelp()
		os.Exit(1)
	}

	// Check if input file exists
	if _, err := os.Stat(*inputFile); os.IsNotExist(err) {
		log.Fatalf("Input file does not exist: %s", *inputFile)
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(*outputFile)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}
	}

	fmt.Printf("Starting domain checker...\n")
	fmt.Printf("Input file: %s\n", *inputFile)
	fmt.Printf("Output file: %s\n", *outputFile)
	fmt.Printf("Workers: %d\n", *workers)
	fmt.Println()

	// Create checker instance
	c := checker.NewChecker(*workers)

	// Read domains from input file
	fmt.Println("Reading domains from input file...")
	domains, err := c.ReadDomains(*inputFile)
	if err != nil {
		log.Fatalf("Failed to read domains: %v", err)
	}

	if len(domains) == 0 {
		log.Fatal("No domains found in input file")
	}

	fmt.Printf("Found %d domains to check\n", len(domains))

	// Check domains
	fmt.Println("Checking domains...")
	results := c.CheckDomains(domains)

	// Count results
	successCount := 0
	failureCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failureCount++
			fmt.Printf("FAILED: %s - %s\n", result.Domain, result.Error)
		}
	}

	// Write successful domains to output file
	if successCount > 0 {
		fmt.Println("Writing successful domains to output file...")
		if err := c.WriteSuccessfulDomains(results, *outputFile); err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
	}

	fmt.Println()
	fmt.Printf("Results:\n")
	fmt.Printf("  Total domains: %d\n", len(domains))
	fmt.Printf("  Successful: %d\n", successCount)
	fmt.Printf("  Failed: %d\n", failureCount)
	fmt.Printf("  Output file: %s\n", *outputFile)
}

func showHelp() {
	fmt.Println("Domain Checker CLI")
	fmt.Println()
	fmt.Println("This tool checks domains for the presence of operate_area field in /__dayu/siteInfo.html")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s -i input.txt -o output.txt [options]\n", os.Args[0])
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -i string    Input file containing domains (one per line) [required]")
	fmt.Println("  -o string    Output file for successful domains [required]")
	fmt.Println("  -w int       Number of concurrent workers (default: 10)")
	fmt.Println("  -h           Show this help message")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Printf("  %s -i domains.txt -o successful_domains.txt -w 20\n", os.Args[0])
	fmt.Println()
}
