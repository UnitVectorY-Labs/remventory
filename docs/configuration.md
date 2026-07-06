# Configuration

Remventory is configured with environment variables. The prototype is intended to run as one application container connected to Postgres and an OpenAI-compatible model endpoint.

## Required for a useful run

| Variable | Purpose | Example |
|---|---|---|
| `DATABASE_URL` | Postgres connection string. | `postgres://remventory:remventory@localhost:5432/remventory?sslmode=disable` |
| `OPENAI_MODEL` | Model name sent to the OpenAI-compatible endpoint. | `gpt-4.1-mini` or `llama3.1` |

## Optional

| Variable | Default | Purpose |
|---|---|---|
| `REMVENTORY_HTTP_ADDR` | `:8080` | Address the web server listens on. |
| `OPENAI_BASE_URL` | `http://localhost:11434/v1` | OpenAI-compatible API base URL. |
| `OPENAI_API_KEY` | empty | API key or token for the model endpoint. |
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
