# Usage

Remventory starts as a Go web server with a Remy-centered web UI, health and readiness checks, category/item/proposal endpoints, and an MCP endpoint.

## Run Locally

Start Postgres, then run:

```sh
export DATABASE_URL='postgres://remventory:remventory@localhost:5432/remventory?sslmode=disable'
export OPENAI_BASE_URL='http://localhost:11434/v1'
export OPENAI_TINY_MODEL='small-instruct-model'
export OPENAI_MAIN_MODEL='general-instruct-model'
export OPENAI_THINKING_MODEL='reasoning-model'
go run .
```

The server listens on `:8080` unless `REMVENTORY_HTTP_ADDR` is set.

Open the web UI:

```text
http://localhost:8080/
```

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

## MCP

MCP clients can connect to the streamable HTTP endpoint:

```text
http://localhost:8080/mcp
```

The initial tool surface includes Remy requests, category reads, proposal creation, proposal confirmation, item listing, and inventory queries. Data-changing requests still produce proposals first.

## Prototype Proposal Flow

Data-changing actions are represented as proposals first. To propose a category:

```sh
curl -X POST http://localhost:8080/api/proposals/category \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Video Games",
    "description": "Physical and digital games",
    "attributes": [
      {"key": "platform", "label": "Platform", "data_type": "text", "required": true},
      {"key": "format", "label": "Format", "data_type": "text"}
    ]
  }'
```

Approve or reject the returned proposal:

```sh
curl -X POST http://localhost:8080/api/proposals/<proposal-id>/decision \
  -H 'Content-Type: application/json' \
  -d '{"approve": true}'
```

After approval, list categories or fetch one category definition:

```sh
curl http://localhost:8080/api/categories
curl http://localhost:8080/api/categories/<category-id>
```

To propose an item add:

```sh
curl -X POST http://localhost:8080/api/proposals/item \
  -H 'Content-Type: application/json' \
  -d '{
    "operation": "create",
    "category_id": "<category-id>",
    "title": "Super Mario Bros. Wonder",
    "attributes": {"platform": "Nintendo Switch", "format": "physical"},
    "quantity": 1
  }'
```

Approve the item proposal with the same proposal decision endpoint, then list items:

```sh
curl 'http://localhost:8080/api/items?category_id=<category-id>'
```

Check whether an item already exists:

```sh
curl -X POST http://localhost:8080/api/query_inventory \
  -H 'Content-Type: application/json' \
  -d '{"query": "Do I already have Super Mario Bros. Wonder?", "category_id": "<category-id>"}'
```

When adding an item through Remy, Remventory checks the relevant category first. If a clear match already exists, Remy proposes a quantity change instead of silently creating a duplicate.

## Docker

Build the single application image:

```sh
docker build -t remventory .
```

Run it with the same environment variables used for local development.
