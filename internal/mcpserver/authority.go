package mcpserver

import (
	"context"
	"encoding/json"
	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"github.com/UnitVectorY-Labs/remventory/internal/intake"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func addAuthorityTools(server *mcpserver.MCPServer, s *intake.Service) {
	server.AddTool(mcp.NewTool("suggest_owner_field_curation", mcp.WithDescription("Ask Remy to compare owner fields with cached linked source facts. Returns suggestions with actual source evidence; does not propose or remove data."), mcp.WithString("item_id")), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		item, err := s.Store.GetItem(ctx, r.GetString("item_id", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out, err := s.CurateFields(ctx, item)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructuredOnly(out), nil
	})
	server.AddTool(mcp.NewTool("get_item_catalogue", mcp.WithDescription("Read approved canonical associations, entity IDs and relationships for an owned item, including the exact association required for an unlink proposal."), mcp.WithString("item_id")), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := s.Store.ItemCanonical(ctx, r.GetString("item_id", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructuredOnly(out), nil
	})

	server.AddTool(mcp.NewTool("propose_catalogue_change", mcp.WithDescription("Propose linking an already source-resolved catalogue identity, or unlinking an association. No identification job required. Does not commit changes; explicit approval is required."), mcp.WithString("operation"), mcp.WithString("item_id"), mcp.WithString("provider"), mcp.WithString("kind"), mcp.WithString("id"), mcp.WithString("entity_id"), mcp.WithString("relation")), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := s.Store.ProposeCatalogue(ctx, store.CatalogueProposal{Operation: r.GetString("operation", ""), ItemID: r.GetString("item_id", ""), Ref: authority.Ref{Provider: r.GetString("provider", ""), Kind: r.GetString("kind", ""), ID: r.GetString("id", "")}, EntityID: r.GetString("entity_id", ""), Relation: r.GetString("relation", "identified_as")})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructuredOnly(p), nil
	})

	server.AddTool(mcp.NewTool("search_authority", mcp.WithDescription("Search enabled open authorities. Returns source identifiers; never invent identifiers."), mcp.WithString("provider"), mcp.WithString("text"), mcp.WithString("title"), mcp.WithString("artist"), mcp.WithString("barcode"), mcp.WithString("catalog"), mcp.WithString("format")), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := s.Providers.Provider(r.GetString("provider", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(r.GetArguments())
		var q authority.Query
		_ = json.Unmarshal(b, &q)
		out, err := p.Search(ctx, q)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructuredOnly(out), nil
	})
	server.AddTool(mcp.NewTool("resolve_authority", mcp.WithDescription("Validate an authority identifier and cache source metadata and relationships. Does not change owned inventory or approve associations."), mcp.WithString("provider"), mcp.WithString("kind"), mcp.WithString("id")), func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := s.Resolve(ctx, authority.Ref{Provider: r.GetString("provider", ""), Kind: r.GetString("kind", ""), ID: r.GetString("id", "")}, false)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructuredOnly(out), nil
	})
}
