# Remventory Prototype Technical Product Specification

**Product name:** Remventory  
**Primary agent/personality:** Remy  
**Document purpose:** Define prototype requirements and technical design for the first working implementation.  
**Prototype stance:** Prove the agent-first interaction model before optimizing scale, authentication, file storage, or advanced search.  
**Explicit prototype non-goal:** Do not build a web-based spreadsheet or a traditional serial chat-history application.

---

## 1. Product Purpose

Remventory is a generic, self-hostable inventory management application where the primary interaction model is agent-driven. The user works with Remy, an AI agent/personality, to define what they want to track, add items, query whether items exist, and view dynamically generated inventory interfaces.

The prototype must validate whether Remy can make inventory management feel flexible, pleasant, and useful without becoming either a spreadsheet clone or a plain chat transcript.

Core ideas:

- Inventory can be anything, including LEGO, video games, collectibles, household items, equipment, or other user-defined domains.
- Remy interprets natural language, proposes structured inventory changes, and presents human-in-the-loop confirmation UI before committing changes.
- The MCP server exposes the inventory capabilities used by both the built-in web UI and remote MCP clients.

---

## 2. Prototype Scope

The initial prototype must focus on the minimum complete loop needed to test the Remy interaction model.

### In Scope

- Single Go application.
- Single self-contained Docker container for the application.
- Postgres database.
- OpenAI-compatible endpoint for local or remote LLMs.
- MCP server implemented with [`github.com/mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go).
- Agent layer implemented with [`github.com/google/adk-go`](https://github.com/google/adk-go).
- Trial of AG-UI using [`github.com/ag-ui-protocol/ag-ui`](https://github.com/ag-ui-protocol/ag-ui) for dynamically generated agent interfaces.
- Web UI centered around Remy.
- Category creation.
- Category attributes.
- Item creation from natural language.
- Listing items by category.
- Querying whether an item exists.
- Human-in-the-loop proposal and confirmation flow for all data-changing actions.

### Out of Scope for Initial Prototype

- Full authentication and authorization.
- Multi-user collaboration.
- RAG, embeddings, and vector search.
- File uploads.
- Image storage.
- S3-compatible object storage implementation.
- Traditional spreadsheet-first inventory UI.
- Traditional serial chat-history UI as the main output surface.

---

## 3. Required Technology Choices

These choices are requirements for the prototype.

| Area | Required Choice | Notes |
|---|---|---|
| Application language | Go | The app server, MCP server, agent integration, web backend, and persistence code should be implemented in Go. |
| Deployment | Single Docker container | The application should be packaged as one container image. |
| Database | Postgres | Used for users, categories, attributes, items, proposals, settings, and agent events if needed. |
| MCP server | `github.com/mark3labs/mcp-go` | MCP is central to the architecture and must not be treated as a later integration. |
| Agent layer | `github.com/google/adk-go` | Remy should be implemented through the Go ADK agent layer. |
| LLM backend | OpenAI-compatible API | Must support local LLMs and remote compatible endpoints. |
| Dynamic agent UI | `github.com/ag-ui-protocol/ag-ui` | Worth trying for structured agent-to-frontend UI events. |
| Future object storage | S3-compatible backend | Not implemented in the prototype, but the architecture should not block later image/file support. |

---

## 4. User Experience Requirements

The web UI is the primary prototype surface.

The UI should contain an input area for interacting with Remy, but the output should not be a normal chat log. The main content area should be a dynamically generated interface that reflects what Remy is doing, what Remy found, or what Remy proposes the user should approve.

| ID | Requirement | Priority | Acceptance Criteria |
|---|---|---:|---|
| UX-01 | Provide a Remy-centered web interface. | Must | The user can submit natural-language requests through the web UI and receive Remy-driven responses. |
| UX-02 | Avoid serial chat-history as the main output. | Must | The UI shows generated task/result/proposal interfaces rather than primarily rendering a chat transcript. |
| UX-03 | Render dynamic result interfaces. | Must | For item lookups, category setup, item additions, and list results, the UI displays structured cards, fields, buttons, and item details based on Remy output. |
| UX-04 | Support mobile-friendly generated UI. | Must | Generated UI components are usable on small screens and do not require a spreadsheet layout. |
| UX-05 | Show Remy activity state. | Should | The UI can represent what Remy is doing through a small catalog of states/actions, such as thinking, searching, cataloging, proposing, or confirming. |
| UX-06 | Support Remy personality and voice style. | Should | Remy responses can use an assigned personality/tone. Voice output may be supported later, but the prototype should not block it. |
| UX-07 | Human-in-the-loop confirmation. | Must | Any database-changing action is first presented as a proposal with clear confirm/reject controls. |
| UX-08 | Reject flow feeds back into Remy. | Must | When the user rejects a proposal, the rejection reason can be sent back to Remy so it can revise or take the next step. |

---

## 5. Required Prototype Flows

### F-01: Create Category

The user tells Remy what they want to track.

Example intent:

> I want to track my LEGO sets.

Required behavior:

1. Remy interprets the desired category.
2. Remy suggests a category name.
3. Remy suggests relevant attributes for that category.
4. The UI renders a proposed category definition.
5. The user can approve, reject, or revise the proposal.
6. The category is saved only after approval.

Acceptance criteria:

- A category proposal can be generated from natural language.
- The proposed category includes attribute definitions.
- The category is not committed until the user confirms.

### F-02: Create or Modify Category Attributes

The user can define arbitrary attributes associated with a category.

Examples:

- LEGO might include set number, set name, theme, piece count, year, sealed/opened status.
- Video games might include title, platform/console, region, format, completion status.

Required behavior:

1. Attributes are stored as structured category definitions.
2. Attributes guide item entry and display.
3. Attributes can differ per category.
4. Remy may suggest attributes, but the user must be able to approve or reject them.

Acceptance criteria:

- A category can have an arbitrary set of attributes.
- Each attribute belongs to a specific category.
- The system can list the attributes for a category.
- The system can use category attributes when proposing item records.

### F-03: Add Item

The user describes an item in natural language.

Example intent:

> Add my sealed Super Mario Bros. Wonder for Nintendo Switch.

Required behavior:

1. Remy identifies the likely category.
2. Remy maps the input to the category's attributes.
3. Remy performs a deduplication/existence check before proposing the add.
4. Remy proposes one of the following:
   - Add a new item.
   - Update an existing item.
   - Increase quantity or otherwise resolve a duplicate/variant case.
5. The UI renders the proposed item change.
6. The user approves or rejects the proposal.
7. The database is changed only after approval.

Acceptance criteria:

- Natural language can become a structured item proposal.
- The proposed item includes category-specific attribute values.
- Deduplication is attempted before the proposal is shown.
- Rejection can include a reason that is sent back to Remy.

### F-04: List Items by Category

The user can ask to see items in a category.

Example intent:

> Show me my video games.

Required behavior:

1. Remy identifies the category.
2. The system returns paged items for that category.
3. The UI renders a mobile-friendly generated list or card view.
4. The UI should not require a spreadsheet layout.

Acceptance criteria:

- Items can be listed by category.
- Results are paged.
- Results show category-specific attributes where relevant.

### F-05: Query Item Existence

The user can ask whether they already have an item.

Example intent:

> Do I already have this LEGO set?

Required behavior:

1. Remy identifies the likely category if possible.
2. A query/existence-check flow receives the relevant category definitions.
3. For the prototype, the query helper may receive the full inventory for the relevant category.
4. The LLM returns a structured judgment:
   - `yes`
   - `no`
   - `uncertain`
5. The result includes supporting item details if there are matches or possible matches.
6. The UI renders the result as a generated answer view, not as plain chat history.

Acceptance criteria:

- The user can ask naturally whether an item exists.
- The system responds with a structured yes/no/uncertain answer.
- The UI shows matching or possibly matching item details.

---

## 6. Technical Architecture

The prototype should be structured around one Go application that contains:

- Web server.
- MCP server.
- Agent runtime integration.
- Postgres persistence layer.
- OpenAI-compatible model client configuration.
- Dynamic UI event/state generation.

The MCP server is a central requirement. Remy uses MCP tools internally, and remote MCP clients can use the same server capabilities.

### Logical Architecture

```text
User
  |
  v
Web UI
  |  natural-language request + UI events
  v
Remy Agent Layer using adk-go
  |
  | calls tools
  v
MCP Server using mcp-go
  |
  | reads/writes through proposal workflow
  v
Go Application Services
  |
  v
Postgres

Remote MCP Client
  |
  v
Same MCP Server
```

### Key Architectural Principle

Data-changing agent actions must go through proposals.

Remy should not silently mutate the database. Remy can propose category changes, item additions, item updates, or duplicate-resolution decisions. The user must confirm before those changes are committed.

---

## 7. MCP Server Requirements

The MCP server is the stable tool surface for Remventory. The built-in Remy web UI and external MCP clients should use the same underlying capabilities.

The tool set should be intentionally small, but distinct enough that category management remains structured while inventory add/query interactions remain natural-language friendly.

| ID | Requirement | Priority | Acceptance Criteria |
|---|---|---:|---|
| MCP-01 | Expose MCP server using `mcp-go`. | Must | The server is implemented with `github.com/mark3labs/mcp-go`. |
| MCP-02 | Make the MCP server usable internally and remotely. | Must | Remy can call MCP tools in the web UI, and the same MCP server can be exposed to remote MCP clients. |
| MCP-03 | Provide a universal Remy request tool. | Must | A natural-language request can be sent to Remy through an MCP tool for agent-driven task execution. |
| MCP-04 | Provide structured category definition tools. | Must | Clients can list categories and retrieve category definitions, including attributes. |
| MCP-05 | Provide category management capability. | Must | The system can create category proposals and commit approved category definitions. |
| MCP-06 | Provide natural-language item addition capability. | Must | The system can accept a natural-language item description and produce a proposed item add/update action. |
| MCP-07 | Provide item listing by category. | Must | The system can return paged items for a category. |
| MCP-08 | Provide natural-language inventory query capability. | Must | The system can answer whether an item exists using relevant category/item context. |
| MCP-09 | Separate proposed changes from committed changes. | Must | No data-changing MCP flow directly commits agent-proposed mutations without user confirmation. |

### 7.1 Proposed Initial Tool Surface

The exact function names can change during implementation, but the prototype should preserve these tool responsibilities.

| Tool | Input Style | Writes Data? | Responsibility |
|---|---|---:|---|
| `remy_request` | Natural language plus context | No direct commit | Universal agent-driven entry point for accomplishing a user request through Remy. |
| `list_categories` | Structured | No | Return available categories. |
| `get_category_definition` | Structured | No | Return a category schema, including attribute definitions. |
| `propose_category_change` | Natural language or structured draft | Creates proposal only | Generate a pending proposal for category creation or attribute changes. |
| `propose_item_change` | Natural language item description | Creates proposal only | Generate a pending proposal for item creation, quantity change, or item update. |
| `confirm_proposal` | Structured proposal ID plus decision | Yes, on approve | Commit an approved proposal or record a rejection reason. |
| `list_items` | Structured category/page filters | No | Return paged inventory items for a category. |
| `query_inventory` | Natural language query plus optional category | No | Determine whether matching inventory appears to exist and return supporting details. |

---

## 8. Agent Requirements

Remy is the main agent/personality. The prototype should focus less on a large hierarchy of named sub-agents and more on clear tool use and context construction.

Where internal helper agents are useful, they should serve specific tasks such as:

- Category interpretation.
- Item extraction.
- Duplicate/existence checking.
- Proposal drafting.

| ID | Requirement | Priority | Acceptance Criteria |
|---|---|---:|---|
| A-01 | Implement Remy using `adk-go`. | Must | The agent layer uses `github.com/google/adk-go`. |
| A-02 | Use OpenAI-compatible model endpoints. | Must | Model provider configuration supports local LLMs through OpenAI-compatible APIs. |
| A-03 | Populate Remy context with category definitions. | Must | When Remy handles a request, it receives all category names and category attribute definitions. |
| A-04 | Do not populate Remy with the entire inventory by default. | Must | Full inventory item context is not included in the top-level Remy context unless a specific flow requires it. |
| A-05 | Allow query/existence helper context to include a full category inventory. | Must | For the prototype, a query flow may provide an entire relevant category inventory to the LLM to decide whether an item exists. |
| A-06 | Return structured outputs. | Must | Agent responses that drive UI or data changes are parseable structured outputs, not only prose. |
| A-07 | Generate proposals, not direct mutations. | Must | Remy can propose changes but cannot silently commit them. |
| A-08 | Handle uncertainty explicitly. | Should | When a match or category selection is uncertain, Remy should show uncertainty and ask for confirmation or a choice. |

---

## 9. Dynamic UI and AG-UI Requirements

The prototype should evaluate AG-UI as the protocol for agent-to-frontend interaction.

The goal is not to predefine every possible screen. The system should emit structured UI events/states that the web frontend can render as task-focused, mobile-friendly interfaces.

| ID | Requirement | Priority | Acceptance Criteria |
|---|---|---:|---|
| UI-01 | Trial AG-UI for generated interfaces. | Must | The implementation attempts to use `github.com/ag-ui-protocol/ag-ui` for structured agent/UI events. |
| UI-02 | Represent Remy states as UI events. | Should | The frontend can show Remy activity states such as thinking, searching, cataloging, proposing, confirming, and completed. |
| UI-03 | Render proposal cards. | Must | Proposed category/item changes display as structured cards with attributes, source reasoning/summary, and approve/reject controls. |
| UI-04 | Render item result cards. | Must | Query results display matching items with category, attributes, quantity/status if present, and relevant details. |
| UI-05 | Render paged category item lists. | Must | List results render as navigable item cards or compact lists, not a required spreadsheet grid. |
| UI-06 | Keep generated UI constrained by safe component types. | Must | Remy can choose from supported component payloads. Arbitrary frontend code generation is not required for the prototype. |

### 9.1 Initial Supported UI Payload Types

These are the minimum useful payload types for the prototype.

- Remy activity state.
- Text answer or summary.
- Category proposal card.
- Item proposal card.
- Item result card.
- Paged category item list.
- Confirmation controls.
- Rejection reason input.

---

## 10. Data Model Requirements

Postgres should store a schema-light inventory while preserving enough structure to make category definitions reliable.

Category definitions should define attributes. Items should store category-specific values in JSONB to avoid a rigid global item schema.

| Table | Purpose | Key Fields | Notes |
|---|---|---|---|
| `users` | Future-proof ownership model. | `id`, `display_name`, `created_at` | Prototype can create/use a default user even without full authentication. |
| `categories` | Defines trackable inventory domains. | `id`, `user_id`, `name`, `description`, `created_at`, `updated_at` | Examples: LEGO, video games. |
| `category_attributes` | Defines fields for a category. | `id`, `category_id`, `key`, `label`, `data_type`, `required`, `display_order`, `config_json` | Attributes are structured so Remy and the UI can reason about them. |
| `items` | Stores inventory records. | `id`, `user_id`, `category_id`, `title`, `attributes_jsonb`, `quantity`, `created_at`, `updated_at` | `attributes_jsonb` stores category-specific values. |
| `proposals` | Stores pending/approved/rejected changes. | `id`, `user_id`, `type`, `status`, `proposed_payload_jsonb`, `reason`, `created_at`, `decided_at` | Required for human-in-the-loop commit flow. |
| `agent_events` | Stores recent agent/UI events if needed. | `id`, `user_id`, `session_id`, `event_type`, `payload_jsonb`, `created_at` | Useful for dynamic UI rendering and debugging. Can be limited in prototype. |
| `settings` | Stores app/model/personality configuration. | `id`, `user_id`, `key`, `value_jsonb` | Supports OpenAI-compatible endpoint config and Remy personality. |
| `item_assets` | Future image/file association. | `id`, `item_id`, `object_key`, `mime_type`, `metadata_jsonb` | Not used in prototype. Included only as future path for S3-compatible storage. |

### 10.1 JSONB Usage

- Use JSONB for item attribute values because different categories require different fields.
- Keep category attribute definitions relational so the system can list, validate, display, and prompt for fields predictably.
- Use JSONB for proposal payloads so proposed category/item changes can be reviewed exactly before commit.
- Do not require a rigid global item schema beyond shared fields such as `id`, `user_id`, `category_id`, `title`, `quantity`, and timestamps.

---

## 11. Inventory Query and Deduplication Approach

The prototype should deliberately avoid RAG and vector search.

Instead, it should test whether careful context construction and structured LLM responses can support the core interaction model.

| ID | Requirement | Priority | Acceptance Criteria |
|---|---|---:|---|
| Q-01 | Load all category definitions into Remy context. | Must | Remy can choose or propose categories using category names and attribute definitions. |
| Q-02 | For category-scoped existence checks, allow full category inventory in context. | Must | The query helper can receive all items in a relevant category and return whether the requested item appears to exist. |
| Q-03 | Return structured match judgments. | Must | The query flow returns yes/no/uncertain, matched item IDs, confidence/summary, and explanation suitable for UI display. |
| Q-04 | Use deduplication before item add proposal. | Must | Before proposing an add, Remy checks whether the item may already exist and proposes add/update/quantity handling accordingly. |
| Q-05 | Defer RAG to future scaling work. | Must | No vector database or embedding pipeline is required in the prototype. |

### 11.1 Context Construction Rules

Top-level Remy context should include:

- User's available categories.
- Attribute definitions for every category.
- Remy personality/configuration.
- Current user request.
- Relevant recent proposal or UI state if needed.

Top-level Remy context should not include:

- The full inventory across all categories by default.

Category-scoped query/existence context may include:

- The full inventory for the relevant category.
- Category attribute definitions.
- The user's natural-language query.

---

## 12. Configuration and Deployment Requirements

| ID | Requirement | Priority | Acceptance Criteria |
|---|---|---:|---|
| D-01 | Single Docker container deployment. | Must | The prototype is packaged as one container image for the Go application. |
| D-02 | Postgres connection configuration. | Must | The app accepts database connection settings via environment variables or configuration file. |
| D-03 | OpenAI-compatible endpoint configuration. | Must | The app accepts model base URL, model name, and API key/token configuration. |
| D-04 | Default local/self-hosted posture. | Must | The prototype assumes a self-hosted user who can run the container and connect it to Postgres and a model endpoint. |
| D-05 | Simple access token gate may be used. | Could | The prototype may use a simple token to access the web UI, but full auth is out of scope. |

---

## 13. Phased Implementation Plan

This section is formatted for an AI implementation agent. Each phase should produce working, testable behavior before moving to the next phase.

### Phase 1: Project Skeleton and Configuration

Goal: create the single Go application foundation.

Tasks:

- Create Go project structure.
- Add Dockerfile for one application container.
- Add configuration loading for:
  - Postgres connection.
  - OpenAI-compatible endpoint base URL.
  - OpenAI-compatible API key/token.
  - Model name.
  - Optional simple web access token.
- Add application startup checks.
- Add basic web server health endpoint.

Acceptance criteria:

- The Go application builds.
- The application runs in one Docker container.
- The application can read database and model configuration.
- A health endpoint confirms the server is running.

### Phase 2: Postgres Schema and Persistence Layer

Goal: implement the prototype database model.

Tasks:

- Add migrations for:
  - `users`
  - `categories`
  - `category_attributes`
  - `items`
  - `proposals`
  - `agent_events`, if needed for the UI/event flow
  - `settings`
  - `item_assets` as future path only, with no upload behavior
- Create or seed a default user for the prototype.
- Implement persistence methods for:
  - Listing categories.
  - Getting category definitions.
  - Creating pending proposals.
  - Confirming or rejecting proposals.
  - Creating categories from approved proposals.
  - Creating/updating items from approved proposals.
  - Listing items by category.

Acceptance criteria:

- The database schema exists.
- The app can use a default user.
- Category definitions can be stored and loaded.
- Items can be stored with category-specific JSONB attributes.
- Proposals can be stored, approved, and rejected.

### Phase 3: MCP Server Foundation

Goal: expose the core capability boundary through MCP using `mcp-go`.

Tasks:

- Add MCP server using `github.com/mark3labs/mcp-go`.
- Implement initial tools:
  - `list_categories`
  - `get_category_definition`
  - `propose_category_change`
  - `propose_item_change`
  - `confirm_proposal`
  - `list_items`
  - `query_inventory`
  - `remy_request`
- Ensure data-changing flows create proposals rather than directly mutating data.
- Ensure `confirm_proposal` is the only tool that commits approved proposed changes.

Acceptance criteria:

- MCP tools can be called internally by the application.
- The MCP server can be exposed to remote MCP clients.
- Category and item read tools return structured data.
- Proposal tools do not silently commit mutations.

### Phase 4: Remy Agent Integration

Goal: implement Remy using `adk-go` and an OpenAI-compatible endpoint.

Tasks:

- Add agent integration using `github.com/google/adk-go`.
- Configure the model client against an OpenAI-compatible endpoint.
- Implement Remy's system behavior:
  - Generic inventory domain.
  - Agent-first interaction.
  - Human-in-the-loop proposals.
  - Structured outputs for UI and tool calls.
- Populate Remy's context with:
  - All categories.
  - All category attribute definitions.
  - User request.
  - Relevant proposal/UI state when needed.
- Do not include the entire inventory in Remy's default context.
- Implement category-scoped query helper behavior that may receive the full inventory for a relevant category.

Acceptance criteria:

- Remy can receive a natural-language request.
- Remy can choose or propose a category using category definitions.
- Remy can produce structured proposed changes.
- Remy does not silently commit database changes.
- The model endpoint can be local or remote as long as it is OpenAI-compatible.

### Phase 5: Dynamic Web UI with AG-UI Trial

Goal: build the Remy-centered web UI without turning it into a chat transcript or spreadsheet.

Tasks:

- Build a web UI with:
  - A primary input area for Remy requests.
  - A main generated interface area.
  - Remy activity/state representation.
- Trial `github.com/ag-ui-protocol/ag-ui` for structured agent/UI events.
- Support rendering for:
  - Remy activity states.
  - Text answer/summary.
  - Category proposal card.
  - Item proposal card.
  - Item result card.
  - Paged category item list.
  - Confirm/reject controls.
  - Rejection reason input.
- Keep UI generation constrained to supported component payloads.
- Do not generate arbitrary frontend code from the model.

Acceptance criteria:

- The user interacts with Remy through the web UI.
- The output is a generated task/result/proposal UI, not a serial chat history.
- Proposal cards include clear approve/reject actions.
- Item and category results are mobile-friendly.

### Phase 6: Category and Attribute Flow

Goal: complete the first core Remy workflow.

Tasks:

- Support natural-language category creation requests.
- Have Remy suggest category attributes.
- Store category proposals.
- Render category proposal UI.
- Approve/reject category proposals.
- On approval, commit category and attributes to Postgres.

Acceptance criteria:

- A user can say they want to track something.
- Remy proposes a category and attributes.
- The UI displays the proposal.
- The proposal is committed only after approval.

### Phase 7: Item Add and Deduplication Flow

Goal: complete item creation through natural language and proposal confirmation.

Tasks:

- Support natural-language item add requests.
- Identify the relevant category.
- Extract category-specific attribute values.
- Run a category-scoped deduplication/existence check.
- Propose one of:
  - New item add.
  - Existing item update.
  - Quantity adjustment.
  - Uncertain match requiring user confirmation.
- Render item proposal UI.
- Approve/reject item proposals.
- On approval, commit the item add/update to Postgres.

Acceptance criteria:

- A user can add an item through natural language.
- The proposed item contains structured attributes.
- The system attempts deduplication before proposing the change.
- The database changes only after approval.

### Phase 8: Listing and Query Flow

Goal: complete read-focused inventory interactions.

Tasks:

- Implement list items by category.
- Render paged item list UI.
- Implement natural-language existence queries.
- For category-scoped existence checks, pass the full relevant category inventory to the query helper.
- Return structured query results with:
  - `yes`, `no`, or `uncertain`.
  - Matched or possible matched item IDs.
  - Explanation/summary.
  - Supporting item details for UI display.

Acceptance criteria:

- A user can list items in a category.
- A user can ask whether they have an item.
- Query results render as generated UI cards or answer views.
- No RAG, embeddings, or vector database are required.

---

## 14. Prototype Acceptance Criteria

The prototype is acceptable when all of the following are true:

- A user can open the web UI and interact with Remy through a single primary input area.
- A user can tell Remy they want to track a new category, receive suggested attributes, and approve the category creation.
- A user can add an item in natural language, see a structured proposed item record, and approve or reject it.
- A user can list items in a selected category through a generated UI view.
- A user can ask whether they already have an item and receive a structured yes/no/uncertain response with supporting details.
- A proposed data-changing action is not committed until the user confirms it.
- The MCP server exposes the core inventory capabilities and is used by the Remy web experience.
- The implementation uses Go, Postgres, `mcp-go`, `adk-go`, an OpenAI-compatible endpoint, and attempts AG-UI for dynamic agent interfaces.

---

## 15. Explicitly Deferred Questions

These questions are important but should not block the prototype unless they become necessary during implementation.

- Whether categories should support subcategories.
- How full authentication, roles, and multi-user behavior should work.
- How to scale search beyond full category inventory context.
- How image upload and S3-compatible storage should be exposed in the UI.
- How much long-term conversation or agent event history should be retained.

---

## 16. Implementation Guardrails

These guardrails are intended to keep the prototype aligned with the product vision.

- Do not turn the web UI into a spreadsheet clone.
- Do not make serial chat history the primary output surface.
- Do not require the user to manually fill rigid forms as the main interaction path.
- Do not let Remy silently commit data-changing actions.
- Do not introduce RAG, embeddings, or vector search in the initial prototype.
- Do not implement file uploads in the initial prototype.
- Do not optimize for multi-user collaboration in the initial prototype.
- Keep the MCP server central and usable by both Remy and remote MCP clients.
- Keep category definitions structured.
- Keep item attributes flexible through JSONB.
- Keep generated UI constrained to known safe component payloads.

---

## 17. Technical References

- `mcp-go`: <https://github.com/mark3labs/mcp-go>
- Google ADK for Go: <https://github.com/google/adk-go>
- AG-UI protocol: <https://github.com/ag-ui-protocol/ag-ui>
