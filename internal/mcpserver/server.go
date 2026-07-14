package mcpserver

import (
	"context"
	"encoding/json"

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
		mcp.NewTool("propose_category_change",
			mcp.WithDescription("Create a pending proposal to create, update, or delete a category and its attributes. This does not commit data."),
			mcp.WithString("operation", mcp.Description("create, update, or delete.")),
			mcp.WithString("category_id", mcp.Description("Existing category ID for update or delete.")),
			mcp.WithString("name", mcp.Description("Proposed category name.")),
			mcp.WithString("description", mcp.Description("Optional category description.")),
			mcp.WithArray("attributes", mcp.Description("Attribute objects with key, label, data_type, required, and display_order.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			attributes, err := decodeAttributes(request.GetArguments()["attributes"])
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			proposal, err := repo.CreateCategoryProposal(ctx, store.CategoryProposalPayload{
				Operation:   request.GetString("operation", "create"),
				CategoryID:  request.GetString("category_id", ""),
				Name:        request.GetString("name", ""),
				Description: request.GetString("description", ""),
				Attributes:  attributes,
			})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructuredOnly(proposal), nil
		},
	)

	server.AddTool(
		mcp.NewTool("propose_item_change",
			mcp.WithDescription("Create a pending proposal for an item create, update, delete, or quantity adjustment. This does not commit data."),
			mcp.WithString("operation", mcp.Description("create, update, delete, or quantity_adjust.")),
			mcp.WithString("category_id", mcp.Description("Category ID.")),
			mcp.WithString("item_id", mcp.Description("Existing item ID for update or quantity_adjust.")),
			mcp.WithString("title", mcp.Description("Item title.")),
			mcp.WithObject("attributes", mcp.Description("Category-specific attribute values.")),
			mcp.WithNumber("quantity", mcp.Description("Item quantity for create or update.")),
			mcp.WithNumber("quantity_delta", mcp.Description("Quantity delta for quantity_adjust.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			attributes, err := json.Marshal(request.GetArguments()["attributes"])
			if err != nil || string(attributes) == "null" {
				attributes = []byte(`{}`)
			}
			proposal, err := repo.CreateItemProposal(ctx, store.ItemProposalPayload{
				Operation:     request.GetString("operation", "create"),
				CategoryID:    request.GetString("category_id", ""),
				ItemID:        request.GetString("item_id", ""),
				Title:         request.GetString("title", ""),
				Attributes:    attributes,
				Quantity:      request.GetInt("quantity", 1),
				QuantityDelta: request.GetInt("quantity_delta", 0),
			})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructuredOnly(proposal), nil
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
		mcp.NewTool("query_inventory",
			mcp.WithDescription("Determine whether an item appears to exist in inventory and return yes, no, or uncertain with supporting matches."),
			mcp.WithString("query", mcp.Description("Natural-language inventory query.")),
			mcp.WithString("category_id", mcp.Description("Optional category ID.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := remyService.QueryInventory(ctx, request.GetString("query", ""), request.GetString("category_id", ""))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructuredOnly(result), nil
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

func decodeAttributes(value any) ([]store.AttributeDraft, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var attributes []store.AttributeDraft
	if err := json.Unmarshal(raw, &attributes); err != nil {
		return nil, err
	}
	return attributes, nil
}
