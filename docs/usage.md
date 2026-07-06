# Usage

Remventory currently starts as a Go web server with health, readiness, configuration, and category-listing endpoints. The Remy UI, MCP tools, and agent flows will build on this foundation.

## Run Locally

Start Postgres, then run:

```sh
export DATABASE_URL='postgres://remventory:remventory@localhost:5432/remventory?sslmode=disable'
export OPENAI_BASE_URL='http://localhost:11434/v1'
export OPENAI_MODEL='llama3.1'
go run .
```

The server listens on `:8080` unless `REMVENTORY_HTTP_ADDR` is set.

## Check the Server

```sh
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

List categories:

```sh
curl http://localhost:8080/api/categories
```

When `REMVENTORY_ACCESS_TOKEN` is set:

```sh
curl -H "Authorization: Bearer $REMVENTORY_ACCESS_TOKEN" http://localhost:8080/api/categories
```

## Docker

Build the single application image:

```sh
docker build -t remventory .
```

Run it with the same environment variables used for local development.
