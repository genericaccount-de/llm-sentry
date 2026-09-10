# LLM Sentry

A transparent HTTP reverse proxy that scans requests (and, from v1, responses)
to LLM APIs for secrets, PII, and prompt-injection content before they reach
the model provider or an agent's action loop.

See [DESIGN.md](DESIGN.md) for the full design (MVP and v1 scope).

## Layout

- `cmd/sentry` — the proxy binary entrypoint.
- `internal/config` — YAML config loading.
- `internal/proxy` — the HTTP reverse proxy and its metrics.
- `internal/rules` — the regex/keyword detection engine.

## Quick start

```bash
make build
./bin/sentry -config config.example.yaml
```
