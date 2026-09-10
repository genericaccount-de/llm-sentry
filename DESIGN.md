## LLM Sentry — Design Document

### 1. Overview

**Product name (working):** LLM Sentry
**Goal:** A transparent HTTP reverse proxy that sits between an LLM client (a coding agent, support bot, or any app calling an LLM API) and the upstream model provider, inspecting requests and responses for secrets, PII, and prompt-injection content, and blocking or redacting matches before they leave the organization's boundary or reach an agent's action loop.
**Scope:** Split into two increments.

- **Part A — MVP:** single provider (OpenAI-compatible schema), rule-based detection, request-side blocking/redaction, Docker deployment, no dashboard.
- **Part B — v1:** everything explicitly deferred out of MVP — multi-provider support, response/streaming scanning, prompt-injection detection, ML-assisted classification, and a hosted admin dashboard.

Architecturally this mirrors [comply-mail-poc's smtp-proxy](https://github.com/genericaccount-de/comply-mail-poc/blob/main/smtp-proxy/internal/proxy/proxy.go): intercept a transport, extract content, call a scanner, act on the verdict (`pass` / `block` / `redact`), forward — with the same fail-open/fail-closed tradeoff, just over HTTP instead of SMTP.

***

## Part A — MVP

### 2. Objectives and Success Criteria

**Objectives**

- Provide a drop-in reverse proxy for the OpenAI-compatible `POST /v1/chat/completions` endpoint that any existing client can use by changing only its `base_url`.
- Run deterministic (regex/keyword) checks against the outbound request body for secrets and PII before it reaches the model provider.
- Block or redact matches per a configurable policy; otherwise pass the request through unmodified, including streaming responses.
- Keep an auditable record of every decision without storing raw secret/PII values.

**Success Criteria (for MVP)**

- Latency: request-side scanning adds < 50ms p95 for typical prompt sizes; streaming responses are passed through with no added latency (response content is not inspected in the MVP).
- Detection: ≥ 90% recall on a benchmark set of common secret formats (cloud provider key patterns, generic bearer tokens) and basic PII (email, phone, national ID formats).
- Integration: works end-to-end with at least one real client unmodified — e.g. pointing an OpenAI SDK client or an agent framework (LangChain, a coding agent) at the proxy via `base_url` override.
- Deployability: runs as a single Docker container from one YAML config, no external database required.

***

### 3. Functional Requirements

**Guardrails Proxy (core service)**

- Reverse HTTP proxy for the OpenAI chat completions schema; forwards the client's own API key upstream unmodified (the proxy never stores or mints credentials).
- Passes streaming (SSE) responses through untouched — no response-side inspection in MVP (documented limitation, see §6).
- For each request:
  - Run the rule engine (regex/keyword categories: cloud API keys, generic secrets/tokens, PII patterns, a configurable custom keyword list) against the message content.
  - Apply the configured action per matched category: `pass`, `redact` (replace the matched span with a placeholder before forwarding), or `block` (reject the request with an error, never forwarded upstream).
- Structured audit log (JSON lines): timestamp, matched category, action taken, a hash of the matched span (never the raw value), latency.
- Basic metrics (Prometheus-style, analogous to [comply-mail-poc's metrics.go](https://github.com/genericaccount-de/comply-mail-poc/blob/main/smtp-proxy/internal/proxy/metrics.go)): requests total, blocked total, redacted total, scan latency histogram.
- Config file (YAML, env-var interpolation) analogous to [comply-mail-poc's config packages](https://github.com/genericaccount-de/comply-mail-poc/blob/main/smtp-proxy/internal/config/config.go): upstream base URL, rule set path/categories, action per category, fail-open vs. fail-closed toggle.

**Explicitly out of scope for MVP** (all deferred to Part B):

- Support for any provider other than the OpenAI-compatible schema.
- Any inspection or blocking of the model's response content.
- Prompt-injection detection.
- ML/LLM-assisted classification.
- Admin UI, dashboard, or any persistent store beyond local log files.

***

### 4. Non-Functional Requirements

**Security & Privacy**

- The proxy never logs raw secret or PII values — only category and a one-way hash of the matched span, for dedup/audit purposes.
- TLS on both legs (client→proxy, proxy→upstream); the client's API key is forwarded as-is and never persisted.

**Performance**

- Rule-engine scanning is regex-based and must stay well under the 50ms p95 budget for typical prompt sizes (a few KB of text).
- Streaming responses are proxied with no added buffering or latency in MVP, since response content isn't inspected yet.

**Reliability**

- Fail-open vs. fail-closed is a config toggle. Unlike the email POC (which fails open to avoid blocking business mail), this product should default to **fail-closed**, since teams adopting a security proxy generally prefer a blocked request over a silently unscanned one — but must remain configurable.

**Deployability**

- Single static Go binary + Docker image, one YAML config, no external database dependency for MVP.

***

### 5. High-Level Architecture

**Guardrails Proxy (Go binary)**

- HTTP listener presenting an OpenAI-compatible endpoint.
- Rule engine module (regex categories, reusing the pattern-matching approach from [comply-mail-poc's rules/engine.go](https://github.com/genericaccount-de/comply-mail-poc/blob/main/backend/internal/rules/engine.go)).
- Config loader (YAML + env interpolation, same pattern as existing config packages).
- Structured logger + metrics module.

**Client integration**

- Any OpenAI-SDK-compatible client or agent framework points its `base_url` at the proxy; no code changes required beyond that one setting, mirroring how the SMTP proxy is a drop-in relay target.

**Upstream**

- The real provider endpoint (OpenAI for MVP).

***

### 6. MVP Scope and Limitations

- Single provider (OpenAI-compatible schema only).
- Text-only inspection; no multimodal (image/audio) content scanning.
- Request-side scanning only — responses are streamed through unexamined (a model could still leak data pulled from its own context in a response; this is a known, accepted gap for MVP).
- Rule set capped at a modest number of categories (e.g. ~20) shipped as sane defaults, customizable via config.
- No persistent storage beyond local log files — no multi-tenant story; one config file serves one team/deployment.
- No dashboard; auditing is "read the JSON log lines" or scrape the metrics endpoint.

***
***

## Part B — v1

Builds directly on the MVP; every item here was explicitly named as out-of-scope in Part A §3/§6.

### 2. Objectives and Success Criteria

**Objectives**

- Support the major LLM provider APIs, not just OpenAI's schema.
- Inspect and act on model **responses**, including streamed ones, not just requests.
- Detect prompt-injection attempts embedded in tool outputs or other external content flowing into the model, and in the model's own output before it reaches an agent's action loop.
- Reduce false positives/negatives from pure regex matching with an optional ML-assisted classification pass.
- Give a non-technical compliance reviewer a UI to manage policy and review the audit trail, rather than requiring someone to read JSON log lines.

**Success Criteria (for v1)**

- At least two additional provider schemas supported end-to-end (e.g. Anthropic Messages API, Azure OpenAI or Bedrock).
- Response scanning adds a bounded, documented latency overhead even in streaming mode (target: a fixed, configurable chunk-buffer window, not unbounded full-response buffering).
- Prompt-injection detection achieves a documented recall/precision on an internal test corpus of known injection patterns (to be built as part of this phase).
- A pilot user with no engineering background can view flagged events and adjust a policy through the dashboard without touching the YAML config.

***

### 3. Functional Requirements

**Multi-provider support**

- A provider adapter layer that normalizes requests/responses across OpenAI, Anthropic Messages API, and at least one of Azure OpenAI/Bedrock into one internal representation, so the rule engine and injection detector run provider-agnostically.

**Response and streaming inspection**

- Streaming-aware scanning pipeline: responses are buffered in bounded chunks and scanned incrementally rather than passed straight through.
- Policy-driven action on response matches: redact/block mid-stream, or (as a configurable fallback) hold the full response until scanned if a team prefers certainty over incremental delivery.

**Prompt-injection detection**

- A separate signature/heuristic set (distinct from the secrets/PII rule engine) applied specifically to:
  - Tool-result / external-content messages flowing into the model (e.g. "ignore previous instructions", role-confusion patterns, embedded imperative language in fetched documents).
  - The model's own output, before it is handed back to an agent's action loop.

**ML-assisted classification**

- An optional secondary pass — a small local model or a call to a classifier LLM — invoked only on matches the rule engine flags as ambiguous, to cut false positives and catch paraphrased secrets that don't match a fixed regex.

**Hosted dashboard / control plane**

- A separate service (not embedded in the proxy, so the proxy never blocks on dashboard availability) providing:
  - Policy management UI (categories, actions, per-team overrides).
  - Audit log viewer/search backed by persistent storage.
  - Per-team API key/config management, SSO.
  - Webhook/SIEM export for alerts.

***

### 4. Non-Functional Requirements

**Performance**

- Response buffering must respect a bounded memory/latency budget even under streaming; the system should degrade gracefully (e.g. cap the chunk-scan window) rather than add unbounded latency to a stream.

**Scalability**

- The dashboard/audit store needs persistent, multi-tenant storage (e.g. Postgres) — a genuinely new dependency the MVP deliberately avoided.

**Compliance**

- Audit log retention policy, exportable in a form usable as evidence for a customer's own compliance review (the same kind of GDPR/SOC2-style documentation need as [comply-mail-poc's threat model](https://github.com/genericaccount-de/comply-mail-poc/blob/main/docs/threat-model.md), generalized beyond email).

***

### 5. High-Level Architecture

**Provider adapter layer**

- Normalizes OpenAI/Anthropic/Bedrock/Azure request and response schemas into one internal message representation consumed by the rest of the pipeline.

**Streaming-aware scanning pipeline**

- Chunk buffer + incremental matcher, with a policy-driven flush/block decision, sitting on both the request and response paths.

**Injection-detection module**

- A separate rule/heuristic set from the secrets/PII engine, applied to tool-result content and model output specifically.

**Optional ML classifier sidecar**

- A local model or external API call invoked only for adjudicating ambiguous matches — kept out of the hot path for clear-cut cases.

**Control plane**

- A hosted API + Postgres-backed policy/audit store + web dashboard, running as a separate service from the proxy; the proxy reports to it asynchronously so control-plane downtime never blocks live traffic.

***

### 6. v1 Scope and Limitations

- No fine-tuned/custom ML models in v1 — only pluggable existing classifier APIs or small off-the-shelf local models for the ambiguous-match pass.
- No reversible redaction — the original value can't be recovered from a redacted log entry by design (security property, not a gap, but worth stating explicitly).
- Dashboard is SaaS-only in v1; no on-prem/self-hosted control-plane option yet.
- Multimodal content (images, audio) inspection remains out of scope — still text-only across both request and response paths.
