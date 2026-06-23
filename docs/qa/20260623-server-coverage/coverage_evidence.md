# Server unit-test coverage uplift — evidence (2026-06-23)

## BEFORE (plain go test -cover, default non-integration)
```
internal/api      89.0%
internal/config   90.0%
internal/device   70.0%
```

## AFTER
```
ok  	github.com/HelixDevelopment/helix_ota/server/internal/api	(cached)	coverage: 90.5% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/internal/device	(cached)	coverage: 92.6% of statements
ok  	github.com/HelixDevelopment/helix_ota/server/internal/config	(cached)	coverage: 90.0% of statements
```

## go vet (touched pkgs) — clean (no output)
```
(empty = clean)
```

## go test -race (touched pkgs)
```
ok  	github.com/HelixDevelopment/helix_ota/server/internal/device	(cached)
ok  	github.com/HelixDevelopment/helix_ota/server/internal/api	(cached)
```
