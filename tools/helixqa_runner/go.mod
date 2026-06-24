module helix_ota.tools.helixqa_runner

go 1.26

require digital.vasic.helixqa v0.0.0-00010101000000-000000000000

require (
	digital.vasic.challenges v0.0.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.15.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Consumer-side replaces (§11.4.28(C)): helixqa + its own-org deps at submodules/<name>.
// The main module's replaces govern the whole build graph, so llm_orchestrator's
// internal CamelCase ../LLMProvider replace is overridden here by llmprovider→llm_provider.
replace (
	digital.vasic.challenges => ../../submodules/challenges
	digital.vasic.containers => ../../submodules/containers
	digital.vasic.docprocessor => ../../submodules/doc_processor
	digital.vasic.helixqa => ../../submodules/helixqa
	digital.vasic.llmorchestrator => ../../submodules/llm_orchestrator
	digital.vasic.llmprovider => ../../submodules/llm_provider
	digital.vasic.llmsverifier => ../../submodules/llms_verifier/llm-verifier
	digital.vasic.security => ../../submodules/security
	digital.vasic.visionengine => ../../submodules/vision_engine
)
