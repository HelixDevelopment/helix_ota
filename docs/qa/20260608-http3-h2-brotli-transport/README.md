# QA evidence — HTTP/3 + HTTP/2 dual transport + Brotli

| Field | Value |
|---|---|
| Run id | 20260608-http3-h2-brotli-transport |
| Feature | Locked transport stack (ADR-0004 / master §3): HTTP/3 (QUIC) primary + HTTP/2 fallback via `digital.vasic.http3`; Brotli→gzip→identity content negotiation |
| Date | 2026-06-08 |
| Evidence | [`run.log`](run.log) — real `go test -count=1` output |

## What this proves (anti-bluff, Constitution §11.4)

- **Dual transport**: `TestDualTransportServesH3AndH2` starts the `internal/transport`
  server with a self-signed cert and hits the SAME handler over **a real HTTP/3
  (QUIC) client** (`quic-go/http3.Transport`, asserts `resp.ProtoMajor == 3`) AND
  **a real HTTP/2 client** (the fallback, asserts `resp.ProtoMajor == 2`), both
  returning 200 + identical body. The HTTP/2 response carries `Alt-Svc: h3=…` so
  HTTP/2/1.1 clients can discover and upgrade to HTTP/3.
- **Brotli with fallback**: 5 negotiation tests — Brotli when `br` is offered
  (decoded round-trip), **gzip fallback** when `br;q=0` (the explicit requirement
  for clients that don't support Brotli), gzip-only clients, **identity** for
  unsupported/disabled encodings, and 204/no-body never content-encoded.

## Wiring

- HTTP/3 served via the reusable `digital.vasic.http3` submodule (drop-in
  `net/http.Handler`), consumed via `replace digital.vasic.http3 => ../submodules/http3`.
- `cmd/ota-server` serves HTTP/3+HTTP/2 when `HELIX_TLS_CERT` + `HELIX_TLS_KEY`
  are set (port `HELIX_HTTPS_PORT`, default 8443); plain HTTP otherwise (dev).
- Brotli middleware registered globally in the Gin router.

## Reproduce

```
cd server
go test -count=1 ./internal/transport/ -run TestDualTransportServesH3AndH2 -v
go test -count=1 ./internal/api/ -run TestCompression -v
```
