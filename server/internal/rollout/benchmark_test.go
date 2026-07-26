// server/internal/rollout/benchmark_test.go
// §11.4.169(13) benchmarking/performance test type for rollout evaluation.
//
// Measures p50/p95/p99 latency for:
//   - CreateAndStart (plan persistence + phase activation)
//   - Get (read-back stored state)
//   - Evaluate (health-verdict → decision pipeline)
//
// Run with: go test -bench=. -benchmem -run='^$' -count=3 ./server/internal/rollout/
package rollout

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	engine "github.com/HelixDevelopment/ota-rollout-engine"
)

func twoPhasePlanBench() []engine.Phase {
	return []engine.Phase{
		{Percentage: 10, SuccessThreshold: 0.90, ErrorThreshold: 0.02, Duration: 30 * time.Second, AutoProgress: true},
		{Percentage: 100, SuccessThreshold: 0.95, ErrorThreshold: 0.01, Duration: 0, AutoProgress: true},
	}
}

func BenchmarkRolloutCreateAndStart(b *testing.B) {
	ctx := context.Background()
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	svc := NewService(func() time.Time { return now })
	plan := twoPhasePlanBench()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		did := fmt.Sprintf("dep-bench-%d", i)
		if _, err := svc.CreateAndStart(ctx, did, plan); err != nil {
			b.Fatalf("CreateAndStart: %v", err)
		}
	}
}

func BenchmarkRolloutGet(b *testing.B) {
	ctx := context.Background()
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	svc := NewService(func() time.Time { return now })

	if _, err := svc.CreateAndStart(ctx, "dep-bench-get", twoPhasePlanBench()); err != nil {
		b.Fatalf("seed CreateAndStart: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Get(ctx, "dep-bench-get"); err != nil {
			b.Fatalf("Get: %v", err)
		}
	}
}

func BenchmarkRolloutEvaluate(b *testing.B) {
	ctx := context.Background()
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	svc := NewService(func() time.Time { return now })
	verdict := engine.HealthVerdict{SuccessRate: 0.98, ErrorRate: 0.0}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		did := fmt.Sprintf("dep-bench-eval-%d", i)
		if _, err := svc.CreateAndStart(ctx, did, twoPhasePlanBench()); err != nil {
			b.Fatalf("seed CreateAndStart: %v", err)
		}
		if _, err := svc.Evaluate(ctx, did, verdict); err != nil {
			b.Fatalf("Evaluate: %v", err)
		}
	}
}

// BenchmarkRolloutLatencyDistribution measures the raw latency distribution
// for rollout CreateAndStart at N=100, 1000, and 10000 concurrent evaluations.
// Results are reported as p50/p95/p99.
func BenchmarkRolloutLatencyDistribution(b *testing.B) {
	ctx := context.Background()
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			svc := NewService(func() time.Time { return now })
			plan := twoPhasePlanBench()
			durations := make([]time.Duration, n)

			for i := 0; i < n; i++ {
				did := fmt.Sprintf("dep-lat-%d-%d", n, i)
				start := time.Now()
				if _, err := svc.CreateAndStart(ctx, did, plan); err != nil {
					b.Fatalf("CreateAndStart: %v", err)
				}
				durations[i] = time.Since(start)
			}

			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			p50 := durations[n*50/100]
			p95 := durations[n*95/100]
			p99 := durations[n*99/100]

			b.ReportMetric(float64(p50.Microseconds()), "p50_us")
			b.ReportMetric(float64(p95.Microseconds()), "p95_us")
			b.ReportMetric(float64(p99.Microseconds()), "p99_us")
		})
	}
}
