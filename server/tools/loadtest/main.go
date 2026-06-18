// Command loadtest is a standalone, black-box HTTP load / NFR measurement harness
// for the Helix OTA control plane (or any HTTP endpoint).
//
// It deliberately depends ONLY on the Go standard library so it compiles and runs
// independently of the server's internal packages — it speaks plain HTTP to a
// running ota-server like any external client would.
//
// All reported numbers (p50/p90/p99 latency, RPS, error count) are MEASURED from
// real request/response round-trips. Nothing is assumed or hard-coded. Per the
// Helix Constitution §11.4 anti-bluff covenant: this tool never asserts an
// unmeasured NFR target — it only reports what it observed.
//
// Usage:
//
//	loadtest -url http://127.0.0.1:8080 -path /healthz -concurrency 50 -duration 10s
//	loadtest -selftest   # spins up a throwaway in-process 200-OK server and measures it
//
// Side-effects: none beyond outbound HTTP requests to the target (and, in
// -selftest mode, a loopback listener that is torn down on exit).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// result is one completed request's outcome.
type result struct {
	latency time.Duration
	err     bool
	status  int
}

// report is the measured summary emitted as JSON + table.
type report struct {
	Target        string  `json:"target"`
	Concurrency   int     `json:"concurrency"`
	DurationSec   float64 `json:"duration_seconds"`
	TotalRequests int64   `json:"total_requests"`
	Errors        int64   `json:"errors"`
	Non2xx        int64   `json:"non_2xx"`
	RPS           float64 `json:"requests_per_second"`
	P50Ms         float64 `json:"p50_ms"`
	P90Ms         float64 `json:"p90_ms"`
	P99Ms         float64 `json:"p99_ms"`
	MinMs         float64 `json:"min_ms"`
	MaxMs         float64 `json:"max_ms"`
	MeanMs        float64 `json:"mean_ms"`
}

func main() {
	var (
		url         = flag.String("url", "http://127.0.0.1:8080", "base URL of the running ota-server")
		path        = flag.String("path", "/healthz", "request path appended to -url")
		concurrency = flag.Int("concurrency", 50, "number of concurrent worker goroutines")
		duration    = flag.Duration("duration", 10*time.Second, "how long to apply load (e.g. 10s, 1m)")
		selftest    = flag.Bool("selftest", false, "run against a throwaway in-process 200-OK server")
		timeout     = flag.Duration("timeout", 30*time.Second, "per-request timeout")
	)
	flag.Parse()

	target := *url + *path

	if *selftest {
		// Spin up a throwaway in-process server returning 200. This proves the
		// harness produces real percentile output end-to-end with no external deps.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
		defer srv.Close()
		target = srv.URL + *path
		if *duration == 0 {
			*duration = 3 * time.Second
		}
		fmt.Fprintf(os.Stderr, "[selftest] measuring throwaway in-process server at %s\n", target)
	}

	if *concurrency < 1 {
		fmt.Fprintln(os.Stderr, "error: -concurrency must be >= 1")
		os.Exit(2)
	}
	if *duration <= 0 {
		fmt.Fprintln(os.Stderr, "error: -duration must be > 0")
		os.Exit(2)
	}

	rep := run(target, *concurrency, *duration, *timeout)

	// JSON output (machine-readable, for CI / further processing).
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rep); err != nil {
		fmt.Fprintln(os.Stderr, "error encoding report:", err)
		os.Exit(1)
	}

	// Human-readable table.
	printTable(rep)
}

// run applies concurrent load for the given duration and returns measured stats.
func run(target string, concurrency int, duration, timeout time.Duration) report {
	// One shared transport with a connection pool sized to the worker count, so
	// we measure server behaviour rather than connection-setup overhead.
	transport := &http.Transport{
		MaxIdleConns:        concurrency * 2,
		MaxIdleConnsPerHost: concurrency * 2,
		MaxConnsPerHost:     concurrency * 2,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	client := &http.Client{Transport: transport, Timeout: timeout}
	defer transport.CloseIdleConnections()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var (
		mu        sync.Mutex
		latencies []time.Duration
		errCount  int64
		non2xx    int64
		total     int64
	)

	// Pre-size per-worker buffers to avoid lock contention on the hot path.
	perWorker := make([][]time.Duration, concurrency)

	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			local := make([]time.Duration, 0, 1024)
			for {
				if ctx.Err() != nil {
					break
				}
				res := doRequest(ctx, client, target)
				atomic.AddInt64(&total, 1)
				if res.err {
					atomic.AddInt64(&errCount, 1)
					continue
				}
				if res.status < 200 || res.status >= 300 {
					atomic.AddInt64(&non2xx, 1)
				}
				local = append(local, res.latency)
			}
			perWorker[idx] = local
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	// Merge per-worker latencies under the lock once (cheap, off the hot path).
	mu.Lock()
	for _, lw := range perWorker {
		latencies = append(latencies, lw...)
	}
	mu.Unlock()

	return summarize(target, concurrency, elapsed, total, errCount, non2xx, latencies)
}

// doRequest performs one GET and measures wall-clock latency including body drain.
func doRequest(ctx context.Context, client *http.Client, target string) result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return result{err: true}
	}
	t0 := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		// A context-cancel at duration boundary is an expected end-of-test event,
		// not a server error — but we still count it as a non-completed request.
		return result{err: true}
	}
	// Drain + close so the connection can be reused (keep-alive).
	drain(resp)
	lat := time.Since(t0)
	return result{latency: lat, status: resp.StatusCode}
}

// drain reads and discards the response body then closes it.
func drain(resp *http.Response) {
	defer resp.Body.Close()
	buf := make([]byte, 32*1024)
	for {
		_, err := resp.Body.Read(buf)
		if err != nil {
			return
		}
	}
}

// summarize computes percentiles and throughput from the measured latencies.
func summarize(target string, concurrency int, elapsed time.Duration, total, errCount, non2xx int64, latencies []time.Duration) report {
	rep := report{
		Target:        target,
		Concurrency:   concurrency,
		DurationSec:   elapsed.Seconds(),
		TotalRequests: total,
		Errors:        errCount,
		Non2xx:        non2xx,
	}
	if elapsed > 0 {
		rep.RPS = float64(total) / elapsed.Seconds()
	}
	if len(latencies) == 0 {
		return rep
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	rep.MinMs = ms(latencies[0])
	rep.MaxMs = ms(latencies[len(latencies)-1])
	rep.P50Ms = ms(percentile(latencies, 0.50))
	rep.P90Ms = ms(percentile(latencies, 0.90))
	rep.P99Ms = ms(percentile(latencies, 0.99))

	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}
	rep.MeanMs = ms(sum / time.Duration(len(latencies)))
	return rep
}

// percentile returns the p-th percentile (0..1) of a pre-sorted slice using the
// nearest-rank method.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	rank := int(p*float64(len(sorted)+1)) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

func ms(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1e6
}

func printTable(r report) {
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "=== MEASURED LOAD TEST RESULTS (no assumed NFR numbers) ===")
	fmt.Fprintf(os.Stderr, "%-22s %s\n", "Target:", r.Target)
	fmt.Fprintf(os.Stderr, "%-22s %d\n", "Concurrency:", r.Concurrency)
	fmt.Fprintf(os.Stderr, "%-22s %.2fs\n", "Duration:", r.DurationSec)
	fmt.Fprintf(os.Stderr, "%-22s %d\n", "Total requests:", r.TotalRequests)
	fmt.Fprintf(os.Stderr, "%-22s %d\n", "Errors (no response):", r.Errors)
	fmt.Fprintf(os.Stderr, "%-22s %d\n", "Non-2xx responses:", r.Non2xx)
	fmt.Fprintf(os.Stderr, "%-22s %.1f\n", "Requests/sec:", r.RPS)
	fmt.Fprintf(os.Stderr, "%-22s %.3f ms\n", "Latency min:", r.MinMs)
	fmt.Fprintf(os.Stderr, "%-22s %.3f ms\n", "Latency mean:", r.MeanMs)
	fmt.Fprintf(os.Stderr, "%-22s %.3f ms\n", "Latency p50:", r.P50Ms)
	fmt.Fprintf(os.Stderr, "%-22s %.3f ms\n", "Latency p90:", r.P90Ms)
	fmt.Fprintf(os.Stderr, "%-22s %.3f ms\n", "Latency p99:", r.P99Ms)
	fmt.Fprintf(os.Stderr, "%-22s %.3f ms\n", "Latency max:", r.MaxMs)
	fmt.Fprintln(os.Stderr, "===========================================================")
}
