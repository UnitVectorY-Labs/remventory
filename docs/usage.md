# Usage

Remventory currently starts as a Go web server with health, readiness, configuration, category, item, and proposal endpoints. The Remy UI, MCP tools, and agent flows will build on this foundation.

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

## Docker

Build the single application image:

```sh
docker build -t remventory .
```

Run it with the same environment variables used for local development.
