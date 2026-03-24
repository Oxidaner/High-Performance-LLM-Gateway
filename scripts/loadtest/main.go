package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type result struct {
	statusCode int
	latency    time.Duration
	err        error
}

func main() {
	var (
		url         = flag.String("url", "http://localhost:8080/v1/chat/completions", "target URL")
		apiKey      = flag.String("api-key", "", "gateway API key")
		model       = flag.String("model", "gpt-4", "chat model")
		prompt      = flag.String("prompt", "hello from loadtest", "user prompt")
		requests    = flag.Int("requests", 200, "total request count")
		concurrency = flag.Int("concurrency", 20, "worker count")
		timeout     = flag.Duration("timeout", 10*time.Second, "per request timeout")
	)
	flag.Parse()

	if *requests <= 0 {
		fmt.Println("requests must be > 0")
		return
	}
	if *concurrency <= 0 {
		fmt.Println("concurrency must be > 0")
		return
	}

	body, _ := json.Marshal(chatRequest{
		Model: *model,
		Messages: []chatMessage{
			{Role: "user", Content: *prompt},
		},
		Stream: false,
	})

	client := &http.Client{Timeout: *timeout}
	jobs := make(chan int)
	results := make(chan result, *requests)

	startedAt := time.Now()
	var completed int64

	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				r := doRequest(client, *url, *apiKey, body)
				results <- r
				atomic.AddInt64(&completed, 1)
			}
		}()
	}

	go func() {
		for i := 0; i < *requests; i++ {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	latencies := make([]float64, 0, *requests)
	statuses := make(map[int]int)
	var errCount int

	for r := range results {
		if r.err != nil {
			errCount++
			continue
		}
		statuses[r.statusCode]++
		latencies = append(latencies, float64(r.latency.Milliseconds()))
	}

	elapsed := time.Since(startedAt)
	total := float64(atomic.LoadInt64(&completed))
	rps := 0.0
	if elapsed.Seconds() > 0 {
		rps = total / elapsed.Seconds()
	}

	sort.Float64s(latencies)
	fmt.Println("== Load Test Result ==")
	fmt.Printf("target: %s\n", *url)
	fmt.Printf("requests: %d\n", *requests)
	fmt.Printf("concurrency: %d\n", *concurrency)
	fmt.Printf("duration: %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("throughput: %.2f req/s\n", rps)
	fmt.Printf("errors: %d\n", errCount)
	fmt.Printf("status_counts: %v\n", statuses)
	if len(latencies) > 0 {
		fmt.Printf("latency_ms avg=%.2f p50=%.2f p95=%.2f p99=%.2f max=%.2f\n",
			avg(latencies),
			percentile(latencies, 50),
			percentile(latencies, 95),
			percentile(latencies, 99),
			latencies[len(latencies)-1],
		)
	}
}

func doRequest(client *http.Client, url, apiKey string, body []byte) result {
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return result{err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	started := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(started)
	if err != nil {
		return result{latency: latency, err: err}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return result{
		statusCode: resp.StatusCode,
		latency:    latency,
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int((p / 100.0) * float64(len(sorted)-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func avg(items []float64) float64 {
	if len(items) == 0 {
		return 0
	}
	var sum float64
	for _, v := range items {
		sum += v
	}
	return sum / float64(len(items))
}
