# AGENTS.md

## Project

Remventory is a single Go application for a self-hosted, agent-first inventory prototype. The app should center Remy, use Postgres for durable state, expose MCP tools as the stable capability boundary, and keep all data-changing agent actions behind proposals that the user approves or rejects.

## Tech Stack

- Go for the server, persistence, MCP, agent integration, and web backend.
- Postgres for users, categories, attributes, items, proposals, settings, and agent events.
- `github.com/mark3labs/mcp-go` for the MCP server.
- Google ADK for Go for Remy. The current Go module import path is `google.golang.org/adk`, even though the upstream repository is `github.com/google/adk-go`.
- OpenAI-compatible model configuration for local or remote LLMs.
- Trial `github.com/ag-ui-protocol/ag-ui` for structured agent/UI events. If the repository does not expose a Go package, keep local event payloads protocol-shaped and document the gap.
- One Docker image for the application container.

## Implementation Rules

- Do not commit field definitions, tag-field suggestions, templates, items, primary-photo selections, or duplicate-resolution changes directly from agent output. Create a proposal first; commit only through an explicit confirmation path.
- Keep the web UI Remy-centered with a visible natural-language composer and dynamic typed results alongside visual browsing. Do not make a spreadsheet clone or a serial chat transcript the primary output.
- Keep a shared, deliberately expanded field vocabulary and item-specific values in JSONB. Tags suggest approved fields; templates copy starting fields, defaults, and tags only at creation. Items have no ongoing template membership. Title, location, and quantity are optional fields, not mandatory core properties. Links are lists of named references.
- Support item photo capture/uploads with S3 originals and generated thumbnails. Do not introduce arbitrary file uploads, RAG, embeddings, vector search, full auth, or multi-user collaboration in this iteration.
- Documentation should explain setup, configuration, and usage. Avoid documenting internal details that are obvious from code.

## Open Questions

- How should tag aliases, related tags, and item relationships be reviewed?
- What access-control model should replace the optional prototype token?
- How long should agent events and proposal history be retained?
- How should durable photo drafts and background identification be queued?
