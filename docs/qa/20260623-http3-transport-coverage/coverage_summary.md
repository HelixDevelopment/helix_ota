# HTTP/3 + Transport Coverage Evidence — 2026-06-23
Go: go version go1.26.2 darwin/arm64
Run: real go test -coverprofile on this Mac (go1.26.2).

## BEFORE
http3 pkg/server         : total:						(statements)		94.5%
http3 internal/testcert  : total:							(statements)	0.0%
server/internal/transport: total: (statements) 94.9%  (measured before profile)

## AFTER
http3 pkg/server         : total:						(statements)		100.0%
http3 internal/testcert  : total:							(statements)	75.0%
server/internal/transport: total: (statements) 100.0%

## TESTS ADDED
- submodules/http3/pkg/server/server_branches_test.go : 2 tests
  * TestStartAfterShutdownReturnsError  -> covers Start s.shutdown branch (server.go:115-118)
  * TestShutdownContextDeadlineExceeded -> covers Shutdown ctx.Done() branch (server.go:160-161) via real h3 server + blocking in-flight QUIC request
- submodules/http3/internal/testcert/testcert_test.go : 3 tests (Generate happy paths: TLS1.3+h3 ALPN, SAN/usage/validity, freshness)
- server/internal/transport/transport_coverage_test.go : 2 tests
  * TestNewWrapsHTTP3ConstructionError -> covers New h3server.New err branch (transport.go:64-66)
  * TestShutdownReturnsHTTP2Error      -> covers Shutdown h2Err!=nil branch (transport.go:98-100) via real h2 in-flight request + short ctx

## UNREACHABLE (honest, §11.4.6)
http3 internal/testcert remaining 25% (lines 30-32,45-47,50-52,55-57) = the 4 err!=nil guards on ecdsa.GenerateKey / x509.CreateCertificate / x509.MarshalECPrivateKey / tls.X509KeyPair. Generate() takes no injectable seams; these fire only on crypto-primitive failure (e.g. crypto/rand exhaustion). Not unit-testable without rewriting production Generate() to accept DI seams — that is coverage-chasing churn, not honest testing, so left uncovered.

## VERIFICATION
go test ./... PASS (http3 full incl chaos/stress). go vet clean both modules.
go test -race -count=2 PASS on pkg/server + internal/testcert + internal/transport (no flakiness, no goroutine leak).
