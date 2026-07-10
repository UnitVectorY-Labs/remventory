# Configuration

Remventory is configured with environment variables. The prototype is intended to run as one application container connected to Postgres and an OpenAI-compatible model endpoint.

`DATABASE_URL`, `OPENAI_MAIN_MODEL`, and `OPENAI_THINKING_MODEL` are required for the application to report ready. `OPENAI_TINY_MODEL` is optional; without it, Remy uses the request text itself for the short on-screen request label.

## Required for a useful run

| Variable | Purpose | Example |
|---|---|---|
| `DATABASE_URL` | Postgres connection string. | `postgres://remventory:remventory@localhost:5432/remventory?sslmode=disable` |
| `OPENAI_MAIN_MODEL` | Non-thinking model for intent classification, category selection, and structured extraction. | `qwen36-35b-a3b-q6kxl-instruct` |
| `OPENAI_THINKING_MODEL` | Thinking model for inventory matching, visible-context questions, and proposal revision. | `qwen36-35b-a3b-q6kxl-generic` |

## Optional

| Variable | Default | Purpose |
|---|---|---|
| `REMVENTORY_HTTP_ADDR` | `:8080` | Address the web server listens on. |
| `OPENAI_BASE_URL` | `http://localhost:11434/v1` | OpenAI-compatible API base URL. |
| `OPENAI_API_KEY` | empty | API key or token for the model endpoint. |
| `OPENAI_TINY_MODEL` | empty | Fast model used only for short display summaries. |
| `OPENAI_MODEL` | empty | Backward-compatible fallback for both main model tiers. Prefer the explicit variables above. |
| `REMVENTORY_ACCESS_TOKEN` | empty | Optional bearer token gate for application API routes. |
| `REMVENTORY_DEFAULT_USER_NAME` | `Remventory User` | Display name for the prototype default user. |
| `REMVENTORY_AUTO_MIGRATE` | `true` | Runs built-in Postgres migrations on startup. |
| `REMVENTORY_LOG_LEVEL` | `info` | One of `debug`, `info`, `warn`, or `error`. |

## Health Checks

- `GET /healthz` returns liveness for the process.
- `GET /readyz` checks whether required backing services are configured and reachable.

If `REMVENTORY_ACCESS_TOKEN` is set, application API routes require:

```text
Authorization: Bearer <token>
```
