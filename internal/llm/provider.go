// Package llm — provider registry.
// Defines ProviderRef and the static priority list for each ModelClass.
//
// REQ-LLM-002: ModelClass enum and provider-agnostic abstractions.
// REQ-LLM-004: Priority list drives retry fallthrough order.
package llm

// ProviderRef identifies a specific model alias on a named provider.
type ProviderRef struct {
	// Provider is one of "anthropic", "openai", "ollama".
	Provider string
	// Model is the LiteLLM model_list alias (e.g. "claude-sonnet-4-6").
	Model string
}

// ProvidersFor returns the ordered provider priority list for a given ModelClass.
// Returns nil for unknown classes.
func ProvidersFor(class ModelClass) []ProviderRef {
	return defaultPriorities[class]
}

// defaultPriorities maps each ModelClass to its provider priority list
// (Anthropic → OpenAI → Ollama per SPEC §3.2).
var defaultPriorities = map[ModelClass][]ProviderRef{
	DeepResearch: {
		{Provider: "anthropic", Model: "claude-opus-4-7"},
		{Provider: "openai", Model: "gpt-4o"},
	},
	Summary: {
		{Provider: "anthropic", Model: "claude-sonnet-4-6"},
		{Provider: "openai", Model: "gpt-4o-mini"},
		{Provider: "ollama", Model: "ollama/llama3.1"},
	},
	Classify: {
		{Provider: "anthropic", Model: "claude-haiku-4-5"},
		{Provider: "openai", Model: "gpt-4o-mini"},
		{Provider: "ollama", Model: "ollama/llama3.1-small"},
	},
	Embed: {
		{Provider: "openai", Model: "text-embedding-3-large"},
	},
}
