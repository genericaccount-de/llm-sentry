// Package config loads the YAML configuration for the LLM Sentry proxy
// (upstream provider URL, rule set, and fail-open/fail-closed behavior),
// with environment variable interpolation.
package config

// Config is the top-level LLM Sentry configuration.
type Config struct {
	// Upstream is the base URL of the real LLM provider API.
	Upstream string `yaml:"upstream"`
	// FailClosed determines whether a scanner error blocks the request
	// (true, the MVP default) or relays it unscanned (false).
	FailClosed bool `yaml:"fail_closed"`
	// RulesPath points to the rule set file used by the rule engine.
	RulesPath string `yaml:"rules_path"`
}
