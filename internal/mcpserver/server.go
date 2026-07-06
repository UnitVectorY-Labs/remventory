package mcpserver

import (
	"context"

	"github.com/UnitVectorY-Labs/remventory/internal/remy"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func New(version string, repo *store.Store, remyService *remy.Service) *mcpserver.MCPServer {
	server := mcpserver.NewMCPServer("remventory", version)

	server.AddTool(
		mcp.NewTool("remy_request",
			mcp.WithDescription("Send a natural-language inventory request to Remy. Data-changing requests return proposals, not committed changes."),
			mcp.WithString("message", mcp.Description("Natural-language request for Remy.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := remyService.Handle(ctx, remy.Request{Message: request.GetString("message", "")})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructuredOnly(result), nil
		},
	)

	server.AddTool(
		mcp.NewTool("list_categories",
			mcp.WithDescription("Return available inventory categories and their attributes."),
			mcp.WithNumber("limit", mcp.Description("Maximum number of categories to return.")),
			mcp.WithNumber("offset", mcp.Description("Number of categories to skip.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			categories, err := repo.ListCategories(ctx, request.GetInt("limit", 50), request.GetInt("offset", 0))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructuredOnly(categories), nil
		},
	)

	server.AddTool(
		mcp.NewTool("get_category_definition",
			mcp.WithDescription("Return one category schema including attributes."),
			mcp.WithString("category_id", mcp.Description("Category ID.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			category, err := repo.GetCategoryDefinition(ctx, request.GetString("category_id", ""))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructuredOnly(category), nil
		},
	)

	server.AddTool(
		mcp.NewTool("confirm_proposal",
			mcp.WithDescription("Approve or reject a pending proposal. Approval is the commit point for data-changing actions."),
			mcp.WithString("proposal_id", mcp.Description("Proposal ID.")),
			mcp.WithBoolean("approve", mcp.Description("True to approve; false to reject.")),
			mcp.WithString("reason", mcp.Description("Optional rejection or decision reason.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			proposal, err := repo.DecideProposal(ctx, request.GetString("proposal_id", ""), store.ProposalDecision{
				Approve: request.GetBool("approve", false),
				Reason:  request.GetString("reason", ""),
			})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructuredOnly(proposal), nil
		},
	)

	server.AddTool(
		mcp.NewTool("list_items",
			mcp.WithDescription("Return paged items for a category."),
			mcp.WithString("category_id", mcp.Description("Category ID.")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of items to return.")),
			mcp.WithNumber("offset", mcp.Description("Number of items to skip.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			items, err := repo.ListItems(ctx, request.GetString("category_id", ""), request.GetInt("limit", 50), request.GetInt("offset", 0))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructuredOnly(items), nil
		},
	)

	return server
}
