# Application Design

Remventory is an agent-first inventory application. The user describes what they want to track or find, and Remy turns that request into structured inventory views and proposals.

## Core Experience

- The user enters natural-language requests through a Remy-centered web UI.
- Results are rendered as generated task, result, list, or proposal views.
- The main output should not be a serial chat log or spreadsheet.
- Mobile layouts should use cards and compact lists rather than wide grids.

## Proposal Workflow

Data-changing actions always use a human-in-the-loop proposal:

1. Remy interprets the request.
2. Remy drafts a category, attribute, item, update, or duplicate-resolution proposal.
3. The UI shows the proposal with clear approve and reject controls.
4. Approval commits the change.
5. Rejection records the reason so Remy can revise or take the next step.

## Inventory Shape

- Categories define what kind of thing is being tracked, such as LEGO sets, video games, collectibles, equipment, or household items.
- Category attributes define fields that matter for that category.
- Items have shared fields such as title and quantity, plus category-specific attribute values.
- Future image and file support should attach assets to items without changing the core inventory flow.

## MCP Boundary

The MCP server is the stable tool surface for Remventory. Remy and remote MCP clients should use the same inventory capabilities:

- Send a natural-language request to Remy.
- List categories.
- Read a category definition.
- Propose category or item changes.
- Confirm or reject proposals.
- List items in a category.
- Query whether an item appears to exist.

## Current Prototype Surface

Before the Remy UI and MCP server are complete, the HTTP API can exercise the same core proposal behavior:

- Create category proposals.
- Approve or reject proposals.
- Read category definitions.
- Create item proposals.
- List items by category.

This is a testing surface for the prototype loop, not the intended final primary user experience.
