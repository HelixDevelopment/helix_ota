# Server remaining-coverage stream — evidence
Date: 2026-06-23
Scope: server/internal/{deviceemu,config,fabric} (api measured, untouched)

## Coverage BEFORE -> AFTER (plain go test ./pkg/ -cover)
deviceemu: 94.0% -> 97.7%
config:    90.0% -> 100.0%
fabric:    96.7% -> 100.0%
api:       90.5% -> 90.5% (untouched, no regression)

## Final per-package measured (re-run):
  deviceemu: 97.7%
  config: 100.0%
  fabric: 100.0%
  api: 90.5%

## vet + race (deviceemu, config, fabric)
  go vet: CLEAN
  go test -race: PASS

## Unreachable defensive code LEFT (deviceemu, honest §11.4.6):
  emulator.go:199  Register json.Marshal(deviceRegistration) err — struct of strings, Marshal cannot fail
  emulator.go:203  Register http.NewRequestWithContext build err — constant POST + well-formed URL
  emulator.go:251  CheckUpdate http.NewRequestWithContext build err — constant GET + well-formed URL
  emulator.go:527  doJSON json.Marshal(body) err — unexported; all callers pass marshalable structs
  emulator.go:533  doJSON http.NewRequestWithContext build err — constant method + well-formed URL
  (config + fabric: 100% — no residual)
