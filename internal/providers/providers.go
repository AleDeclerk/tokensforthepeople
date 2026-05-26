// Package providers holds the static metadata for every LLM provider t4p
// knows about: identifier, display name, signup URL, validation endpoint,
// and how to authenticate the validation request.
//
// Everything user-visible (the wizard's "where to get a key" hints, the
// matrix doc) is rendered from this table so the codebase has a single
// source of truth for provider facts.
package providers

import "github.com/AleDeclerk/tokensforthepeople/internal/routing"

// AuthScheme determines how the validation call attaches the key.
type AuthScheme int

const (
	// AuthBearer sets Authorization: Bearer <key>.
	AuthBearer AuthScheme = iota
	// AuthQueryParam appends ?key=<key> to the URL. Gemini uses this.
	AuthQueryParam
)

// Provider is everything the wizard needs to interact with a single LLM
// service: how to validate the key, where the user signs up, the env var
// name the downstream config files reference.
type Provider struct {
	ID         routing.Provider
	Display    string
	SignupURL  string
	EnvVar     string
	Endpoint   string // GET endpoint that returns 2xx for a valid key with zero cost
	AuthScheme AuthScheme
	// QueryParamName is consulted only when AuthScheme==AuthQueryParam.
	QueryParamName string
}

// All is the canonical, ordered list of providers we support in v1.
// Order here drives the order shown in the wizard's "which keys do you have"
// screen — most-useful-first.
var All = []Provider{
	{
		ID:         routing.ProviderGemini,
		Display:    "Gemini",
		SignupURL:  "https://aistudio.google.com/apikey",
		EnvVar:     "GEMINI_API_KEY",
		Endpoint:   "https://generativelanguage.googleapis.com/v1beta/models",
		AuthScheme: AuthQueryParam,
		// Gemini accepts the key as either header or query param. Query
		// param keeps the validation call dead simple and matches their docs.
		QueryParamName: "key",
	},
	{
		ID:         routing.ProviderGroq,
		Display:    "Groq",
		SignupURL:  "https://console.groq.com/keys",
		EnvVar:     "GROQ_API_KEY",
		Endpoint:   "https://api.groq.com/openai/v1/models",
		AuthScheme: AuthBearer,
	},
	{
		ID:        routing.ProviderOpenRouter,
		Display:   "OpenRouter",
		SignupURL: "https://openrouter.ai/keys",
		EnvVar:    "OPENROUTER_API_KEY",
		// /auth/key returns the user's remaining credit and rate limits, so
		// it doubles as a quota probe. /api/v1/models also works but doesn't
		// require auth, which makes it useless for validating the key.
		Endpoint:   "https://openrouter.ai/api/v1/auth/key",
		AuthScheme: AuthBearer,
	},
	{
		ID:         routing.ProviderOllamaCloud,
		Display:    "Ollama Cloud",
		SignupURL:  "https://ollama.com/settings/keys",
		EnvVar:     "OLLAMA_API_KEY",
		Endpoint:   "https://ollama.com/api/tags",
		AuthScheme: AuthBearer,
	},
	{
		ID:         routing.ProviderCerebras,
		Display:    "Cerebras",
		SignupURL:  "https://cloud.cerebras.ai/platform/api-keys",
		EnvVar:     "CEREBRAS_API_KEY",
		Endpoint:   "https://api.cerebras.ai/v1/models",
		AuthScheme: AuthBearer,
	},
}

// ByID returns the provider with the given ID. The bool reports whether
// it was found; we don't return errors here because the inputs come from
// our own enum, not user input.
func ByID(id routing.Provider) (Provider, bool) {
	for _, p := range All {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}
