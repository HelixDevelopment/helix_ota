// Helix OTA device-claim prototype (dev-tooling, NOT product runtime).
//
// Consumes the session_orchestrator reusable engine BY REFERENCE per the
// §11.4.28(C) depth-1 carve-out: the engine lives under the constitution
// submodule at constitution/submodules/session_orchestrator and is NEVER copied
// nor re-added as a nested own-org submodule of this consumer. A local `replace`
// directive points the module path at the in-tree engine so the build resolves
// with no network fetch (the engine has zero external deps beyond stdlib —
// helix-deps.yaml `deps: []`).
module github.com/HelixDevelopment/helix_ota/tools/device_claim

go 1.22

require github.com/vasic-digital/session_orchestrator v0.0.0

replace github.com/vasic-digital/session_orchestrator => ../../constitution/submodules/session_orchestrator
