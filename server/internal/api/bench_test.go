package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/HelixDevelopment/helix_ota/server/internal/store"
)

// §11.4.27 benchmarking test type — measured ns/op + allocs for representative
// request paths through the real Server.Router(). Run:
//   go test -bench=. -benchmem -run=^$ ./internal/api/

func BenchmarkHealthz(b *testing.B) {
	router, _ := newResilienceServer(b, store.NewMemoryRepository())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = doStressReq(router, http.MethodGet, "/healthz", "", "")
	}
}

func BenchmarkGroupCreate(b *testing.B) {
	router, srv := newResilienceServer(b, store.NewMemoryRepository())
	tok := resilienceAdminToken(b, srv)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = doStressReq(router, http.MethodPost, "/api/v1/groups", tok, fmt.Sprintf(`{"name":"g-%d"}`, i))
	}
}

func BenchmarkGroupList(b *testing.B) {
	router, srv := newResilienceServer(b, store.NewMemoryRepository())
	tok := resilienceAdminToken(b, srv)
	for i := 0; i < 20; i++ {
		_ = doStressReq(router, http.MethodPost, "/api/v1/groups", tok, fmt.Sprintf(`{"name":"g-%d"}`, i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = doStressReq(router, http.MethodGet, "/api/v1/groups", tok, "")
	}
}

// BenchmarkClientUpdateNoDeployment measures the authenticated device update-check
// fast path (no active deployment -> 204), exercising auth + repo lookups.
func BenchmarkClientUpdateNoDeployment(b *testing.B) {
	repo := store.NewMemoryRepository()
	router, srv := newResilienceServer(b, repo)
	devTok, err := srv.signer.Mint("dev-bench", []string{RoleDevice}, srv.cfg.DeviceTokenTTL, srv.now())
	if err != nil {
		b.Fatalf("mint device token: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = doStressReq(router, http.MethodGet, "/api/v1/client/update", devTok, "")
	}
}
