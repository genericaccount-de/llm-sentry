// Package proxy implements the HTTP reverse proxy that accepts
// OpenAI-compatible chat completion requests, scans them via the rule
// engine, and forwards them upstream — blocking or redacting first if a
// rule matches. Streaming responses are passed through unexamined in the
// MVP (see DESIGN.md Part A §6).
package proxy
