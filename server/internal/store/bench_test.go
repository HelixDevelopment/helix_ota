package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// §11.4.27 benchmarking test type — measured ns/op + allocs for hot in-memory
// store paths. Run: go test -bench=. -benchmem -run=^$ ./internal/store/

func BenchmarkMemoryCreateGroup(b *testing.B) {
	repo := NewMemoryRepository()
	ctx := context.Background()
	ts := time.Now()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = repo.CreateGroup(ctx, Group{ID: fmt.Sprintf("g-%d", i), Name: fmt.Sprintf("n-%d", i), CreatedAt: ts})
	}
}

func BenchmarkMemoryFindDelta(b *testing.B) {
	repo := NewMemoryRepository()
	ctx := context.Background()
	ts := time.Now()
	for i := 0; i < 100; i++ {
		_ = repo.CreateDelta(ctx, DeltaArtifact{ID: fmt.Sprintf("d-%d", i),
			BaseArtifactID: fmt.Sprintf("b-%d", i), TargetArtifactID: fmt.Sprintf("t-%d", i), CreatedAt: ts})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repo.FindDelta(ctx, "b-50", "t-50")
	}
}

func BenchmarkMemoryListAudit(b *testing.B) {
	repo := NewMemoryRepository()
	ctx := context.Background()
	ts := time.Now()
	for i := 0; i < 200; i++ {
		_ = repo.AppendAudit(ctx, AuditEntry{ID: fmt.Sprintf("a-%d", i), Action: "DEVICE_REGISTER",
			ResourceType: "device", CreatedAt: ts})
	}
	f := AuditFilter{Limit: 50}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = repo.ListAudit(ctx, f)
	}
}
