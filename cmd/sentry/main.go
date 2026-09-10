// Command sentry runs the LLM Sentry reverse proxy: an OpenAI-compatible
// HTTP endpoint that scans requests for secrets and PII before relaying
// them to the upstream model provider.
package main

func main() {
	// TODO: load config, wire the rule engine into the proxy handler, start
	// the HTTP server.
}
