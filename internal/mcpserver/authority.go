package mcpserver

import (
	"context"
	"encoding/json"
	"github.com/UnitVectorY-Labs/remventory/internal/authority"
	"github.com/UnitVectorY-Labs/remventory/internal/intake"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func addAuthorityTools(server *mcpserver.MCPServer, s *intake.Service) {
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
