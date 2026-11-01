# Configuration

Remventory is configured with environment variables. The prototype is intended to run as one application container connected to Postgres and an OpenAI-compatible model endpoint.

`DATABASE_URL` and `OPENAI_MAIN_MODEL` are required for the application to report ready. The main model runs Remy's tool-using agent over the OpenAI-compatible chat-completions API. `OPENAI_THINKING_MODEL` is optional and remains available to the separate inventory-query endpoint. Image uploads additionally require the S3-compatible settings below.

## Required for a useful run

| Variable | Purpose | Example |
|---|---|---|
| `DATABASE_URL` | Postgres connection string. | `postgres://remventory:remventory@localhost:5432/remventory?sslmode=disable` |
| `OPENAI_MAIN_MODEL` | OpenAI-compatible model used by Remy's agent, tool selection, and presentation selection. | `qwen38-27b-q6kxl-instruct` |

## Optional

| Variable | Default | Purpose |
|---|---|---|
| `REMVENTORY_HTTP_ADDR` | `:8080` | Address the web server listens on. |
| `OPENAI_BASE_URL` | `http://localhost:11434/v1` | OpenAI-compatible API base URL. |
| `OPENAI_API_KEY` | empty | API key or token for the model endpoint. |
| `OPENAI_VISION_MODEL` | empty | Vision-capable chat-completions model for explicit photo identification. Independent of the text model. |
| `OPENAI_VISION_BASE_URL` | `OPENAI_BASE_URL` | Optional separate provider for photo identification. |
| `OPENAI_VISION_API_KEY` | `OPENAI_API_KEY` | Credential for the vision provider. |
| `OPENAI_MODEL` | empty | Backward-compatible fallback for both main model tiers. Prefer the explicit variables above. |
| `OPENAI_THINKING_MODEL` | empty | Optional model for the separate inventory-query endpoint. Remy's agent uses `OPENAI_MAIN_MODEL`. |
| `REMVENTORY_ACCESS_TOKEN` | empty | Optional bearer token gate for application API routes. |
| `REMVENTORY_DEFAULT_USER_NAME` | `Remventory User` | Display name for the prototype default user. |
| `REMVENTORY_AUTO_MIGRATE` | `true` | Runs built-in Postgres migrations on startup. |
| `REMVENTORY_LOG_LEVEL` | `info` | One of `debug`, `info`, `warn`, or `error`. |
| `S3_ENDPOINT` | empty | Base URL for the S3-compatible object-storage service. |
| `S3_REGION` | `us-east-1` | Signing region expected by the storage service. |
| `S3_BUCKET` | empty | Bucket that stores item originals and thumbnails. |
| `S3_ACCESS_KEY_ID` | empty | Storage access-key ID. |
| `S3_SECRET_ACCESS_KEY` | empty | Storage secret access key. |
| `S3_USE_PATH_STYLE` | `true` | Uses path-style bucket URLs, which are commonly required by compatible services. |

Set the endpoint, bucket, access key, and secret to enable image storage; region and path-style settings have defaults. Create the bucket before uploading. Credentials belong in runtime environment configuration such as a gitignored `.env`, never in source control.

## Health Checks

- `GET /healthz` returns liveness for the process.
- `GET /readyz` checks whether required backing services are configured and reachable.

If `REMVENTORY_ACCESS_TOKEN` is set, application API routes require:

```text
Authorization: Bearer <token>
```

## Local development

Run the application with Postgres:

```sh
container system start
container run --name remventory-postgres --detach --publish 127.0.0.1:5432:5432 \
  --env POSTGRES_PASSWORD=remventory --env POSTGRES_DB=remventory postgres:18
DATABASE_URL='postgres://postgres:remventory@localhost:5432/remventory?sslmode=disable' go run .
```

Open `http://localhost:8080`. Migrations run at startup. Configure S3-compatible storage to save photos and the optional vision model to identify them. No text or vision model is needed for manual entry and browsing, although `/readyz` retains the prototype's text-model readiness requirement.

To run the Postgres integration checks, set `TEST_DATABASE_URL` to a development database and run `go test ./...`. Each run creates and drops an isolated test schema.

The optional frontend regression check exercises the visible Remy composer, registered field picker, template defaults, named links, multi-photo entry, and primary-image approval with a mocked API:

```sh
npm install --prefix /tmp/remventory-ui-tests --no-audit --no-fund jsdom
NODE_PATH=/tmp/remventory-ui-tests/node_modules node scripts/ui-regression.cjs
```

This is test tooling only; the application remains a single Go application with embedded static assets.
