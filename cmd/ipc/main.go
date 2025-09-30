package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	concurrency = 256
	timeout     = 5 * time.Second
)

func main() {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	file, err := os.Create("results.txt")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	defer writer.Flush()

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i := 0; i < 256; i++ {
		for j := 0; j < 256; j++ {
			ip := fmt.Sprintf("http://34.101.%d.%d", i, j)
			wg.Add(1)
			sem <- struct{}{}
			go func(ip string) {
				defer wg.Done()
				defer func() { <-sem }()

				fmt.Printf("Checking: %s\n", ip)
				resp, err := client.Get(ip)
				if err != nil {
					return
				}
				defer resp.Body.Close()

				buf := make([]byte, 8192)
				n, _ := resp.Body.Read(buf)
				if strings.Contains(string(buf[:n]), "STATUS_CODES") {
					fmt.Println("Found on:", ip)
					writer.WriteString(ip + "\n")
					writer.Flush()
				}
			}(ip)
		}
	}

	wg.Wait()
}
