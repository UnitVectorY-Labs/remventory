# Usage

Remventory starts as a Go web server with a Remy-centered web UI, health and readiness checks, category/item/proposal endpoints, and an MCP endpoint.

## Run Locally

Start Postgres, then run:

```sh
export DATABASE_URL='postgres://remventory:remventory@localhost:5432/remventory?sslmode=disable'
export OPENAI_BASE_URL='http://localhost:11434/v1'
export OPENAI_MAIN_MODEL='general-instruct-model'
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

## Smoke-Test Remy

With a populated test database and a configured model endpoint, the opt-in scenario script checks exact inventory totals, the requested item-card presentation, and a follow-up comparison that uses item references from the prior turn:

```sh
REMVENTORY_BASE_URL=http://localhost:8080 python3 scripts/live-remy-smoke.py
```

Prepare a disposable, otherwise empty database and seed its known 254 records / 256 units:

```sh
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/smoke-seed.sql
```

The seed refuses to run if it finds any category outside the two smoke collections. The scenario script also requires `REMVENTORY_SMOKE_DISPOSABLE=YES`; it creates and revises one pending quantity proposal, and never approves it. Reset or discard the disposable database after the run.

Run the app and script once with each model alias as the main agent model, resetting the seed between runs:

Start the application in a terminal with the first alias:

```sh
OPENAI_BASE_URL=https://llm.unitvectory-labs.net/v1 OPENAI_MAIN_MODEL=qwen38-27b-q6kxl-instruct go run .
```

In another terminal, run the scenario:

```sh
REMVENTORY_SMOKE_DISPOSABLE=YES REMVENTORY_BASE_URL=http://localhost:8080 python3 scripts/live-remy-smoke.py
```

Stop the first app, refresh the fixture, then start it with the generic alias and rerun the script:

```sh
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f scripts/smoke-seed.sql
OPENAI_BASE_URL=https://llm.unitvectory-labs.net/v1 OPENAI_MAIN_MODEL=qwen38-27b-q6kxl-generic go run .
```

The app uses the main model for tool and presentation decisions; the generic model's separate `reasoning_content` field is ignored. The scenario checks 254 records and 256 units, retrieves `Archive Game 250` past the prior 200-item bound, chooses item cards and comparison views, and revises a proposal while it remains pending.

## MCP

MCP clients can connect to the streamable HTTP endpoint:

```text
http://localhost:8080/mcp
```

The MCP tool surface includes paged category and item reads, server-side inventory search and aggregation, Remy requests with a reusable session ID, category and item proposals, in-place proposal revision, and explicit proposal confirmation. Every data-changing action still produces a proposal first.

## Working with Remy

Use natural language with Remy as the primary way to search, compare, and propose inventory changes. Select **Browse inventory** for the supporting category and item browser. Press Enter to send and Shift+Enter for a new line. Remy's dialog shows the current work state; the edit icon starts a fresh conversation without changing inventory.

Remy shows category attributes and item values in tables. When proposing an item, it only includes details stated in the request (or already stored on an item being updated); missing details remain blank rather than being guessed. Approve or reject the proposal in the page—rejection does not change inventory.

Every saved item can have one picture in the current UI. After an item-add proposal is approved, Remventory opens the saved item's detail view automatically. Item names in inventory lists open that same view. Drop a JPEG, PNG, or GIF up to 12 MB onto the picture area, or select the area to use the native file chooser. Remventory stores the untouched original and a server-generated 320×320 center-cropped thumbnail under UUID-based object keys. The browser only loads images through the Go application; bucket URLs and credentials are never exposed. Select a saved picture to view the original in a modal. The database relation already supports multiple images per item for a future UI expansion.

Category fields support `text`, `number`, `boolean`, `date`, and `enum`. Enumeration definitions use `config.options`, for example:

```json
{"key":"condition","label":"Condition","data_type":"enum","config":{"options":["New","Good","Fair"]}}
```

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

Upload a picture directly through the API with multipart form data:

```sh
curl -X POST "http://localhost:8080/api/items/<item-id>/images" \
  -F "image=@picture.png"
```

The response includes application-local `thumbnail_url` and `original_url` values. Direct bucket access is intentionally not part of the API.

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
