# Usage

Remventory starts as a Go web server with a Remy-centered web UI, health and readiness checks, category/item/proposal endpoints, and an MCP endpoint.

## Run Locally

Start Postgres, then run:

```sh
export DATABASE_URL='postgres://remventory:remventory@localhost:5432/remventory?sslmode=disable'
export OPENAI_BASE_URL='http://localhost:11434/v1'
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

The MCP tool surface includes category reads, category create/update/delete proposals, item create/update/delete/quantity proposals, proposal confirmation, item listing, inventory queries, and Remy requests. Every data-changing action still produces a proposal first.

## Working with Remy

Use the composer to create, update, remove, or browse inventory. Press Enter to send and Shift+Enter for a new line. Remy's top dialog shows what he is working on and briefly comments on the result. The edit icon starts a fresh chat without changing inventory.

Remy shows category attributes and item values in tables. When proposing an item, it only includes details stated in the request (or already stored on an item being updated); missing details remain blank rather than being guessed. Approve or reject the proposal in the page—rejection does not change inventory.

Inventory questions without an explicit category search across every relevant collection and group matching items by collection. Supplying a category to the query API or MCP tool keeps the search scoped to that category.

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

To replace a category definition (including adding, changing, or removing attributes), create an update proposal with the complete resulting attribute list:

```sh
curl -X POST http://localhost:8080/api/proposals/category \
  -H 'Content-Type: application/json' \
  -d '{
    "operation": "update",
    "category_id": "<category-id>",
    "name": "Video Games",
    "description": "Physical and digital games",
    "attributes": [
      {"key": "platform", "label": "Platform", "data_type": "text", "required": true},
      {"key": "condition", "label": "Condition", "data_type": "text"}
    ]
  }'
```

Set `operation` to `delete` with a `category_id` to propose deleting a category. Approving that proposal also deletes its items. Item proposals likewise accept `update` and `delete`; updates require the current `item_id`, and all operations require approval before they are applied.

## Container image

Build the single application image:

```sh
container build -t remventory .
```

Run it with the same environment variables used for local development.
