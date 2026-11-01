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

	server.AddTool(mcp.NewTool("get_inventory_vocabulary", mcp.WithDescription("Read approved shared fields, tag field suggestions, and creation templates.")), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		vocabulary, err := repo.Vocabulary(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructuredOnly(vocabulary), nil
	})
	server.AddTool(mcp.NewTool("propose_graph_change", mcp.WithDescription("Propose an intentional vocabulary expansion or tag field suggestions. Requires explicit approval."), mcp.WithString("operation", mcp.Description("define_field or suggest_fields")), mcp.WithString("key"), mcp.WithString("label"), mcp.WithString("data_type", mcp.Description("text, number, boolean, date, links, enum")), mcp.WithObject("config"), mcp.WithString("tag"), mcp.WithArray("field_keys")), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := json.Marshal(request.GetArguments())
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var payload store.GraphProposalPayload
		if err := json.Unmarshal(data, &payload); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		proposal, err := repo.CreateGraphProposal(ctx, payload)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructuredOnly(proposal), nil
	})

	server.AddTool(
		mcp.NewTool("remy_request",
			mcp.WithDescription("Send a natural-language inventory request to Remy. Data-changing requests return proposals, not committed changes."),
			mcp.WithString("message", mcp.Description("Natural-language request for Remy.")),
			mcp.WithString("session_id", mcp.Description("Optional UUID conversation ID to continue a prior Remy request.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := remyService.Handle(ctx, remy.Request{Message: request.GetString("message", ""), SessionID: request.GetString("session_id", "")})
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructuredOnly(result), nil
		},
	)

	server.AddTool(mcp.NewTool("browse_inventory_tags", mcp.WithDescription("List inventory tags and exact counts for browsing.")), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := repo.LibraryFacets(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultStructuredOnly(result), nil
	})
	server.AddTool(
		mcp.NewTool("list_categories",
			mcp.WithDescription("Return creation templates with starter fields, defaults, and tags."),
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
			mcp.WithDescription("Return a creation template, its starter fields, defaults, and tags."),
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
		mcp.NewTool("search_inventory",
			mcp.WithDescription("Search all inventory server-side. Returns a bounded page, exact total, and has_more."),
			mcp.WithString("query", mcp.Description("Optional title, tag, location, or attribute text.")),
			mcp.WithString("tags", mcp.Description("Optional comma-separated exact tags; an item must have all of them.")),
			mcp.WithString("location", mcp.Description("Optional exact physical location.")),

			mcp.WithString("attribute_key", mcp.Description("Optional exact attribute key.")),
			mcp.WithString("attribute_value", mcp.Description("Optional exact attribute value.")),
			mcp.WithNumber("limit", mcp.Description("Page size, up to 100.")),
			mcp.WithNumber("offset", mcp.Description("Page offset.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := remyService.SearchTagged(ctx, request.GetString("query", ""), request.GetString("category_id", ""), request.GetString("attribute_key", ""), request.GetString("attribute_value", ""), request.GetString("tags", ""), request.GetString("location", ""), request.GetInt("limit", 50), request.GetInt("offset", 0))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructuredOnly(result), nil
		},
	)
	server.AddTool(
		mcp.NewTool("aggregate_inventory",
			mcp.WithDescription("Compute exact deterministic record/unit totals, numeric min/max, or grouped counts using server-side SQL."),
			mcp.WithString("metric", mcp.Description("records, units, min, max, or group.")), mcp.WithString("attribute_key", mcp.Description("Numeric attribute for min/max.")), mcp.WithString("group_by", mcp.Description("Attribute key for group metric.")), mcp.WithString("filter_key", mcp.Description("Optional exact filter attribute.")), mcp.WithString("filter_value", mcp.Description("Optional exact filter value.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := remyService.Aggregate(ctx, request.GetString("category_id", ""), request.GetString("metric", ""), request.GetString("attribute_key", ""), request.GetString("group_by", ""), request.GetString("filter_key", ""), request.GetString("filter_value", ""))
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructuredOnly(result), nil
		},
	)

	server.AddTool(
		mcp.NewTool("propose_category_change",
			mcp.WithDescription("Propose creating, updating, or deleting a creation template using approved shared fields. This does not commit data."),
			mcp.WithString("operation", mcp.Description("create, update, or delete.")),
			mcp.WithString("category_id", mcp.Description("Existing category ID for update or delete.")),
			mcp.WithString("name", mcp.Description("Proposed category name.")),
			mcp.WithString("description", mcp.Description("Optional template description.")),
			mcp.WithArray("default_tags", mcp.Description("Tags copied when creating an item from this template.")),
			mcp.WithObject("default_values", mcp.Description("Initial values for registered fields.")),
			mcp.WithArray("attributes", mcp.Description("Attribute objects with key, label, data_type (text, number, boolean, date, or enum), required, display_order, and config. Enum config must contain an options array.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			attributes, err := decodeAttributes(request.GetArguments()["attributes"])
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			var defaultTags []string
			if raw, ok := request.GetArguments()["default_tags"]; ok {
				data, _ := json.Marshal(raw)
				if err := json.Unmarshal(data, &defaultTags); err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
			}
			defaults, _ := json.Marshal(request.GetArguments()["default_values"])
			if string(defaults) == "null" {
				defaults = nil
			}
			proposal, err := remyService.ProposeCategoryChange(ctx, store.CategoryProposalPayload{DefaultTags: defaultTags, DefaultValues: defaults,
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
			mcp.WithDescription("Propose item creation, update, deletion, or primary-picture selection. Shared fields are optional. This never commits data."),
			mcp.WithString("operation", mcp.Description("create, update, or delete.")),
			mcp.WithString("category_id", mcp.Description("Category ID.")),
			mcp.WithString("item_id", mcp.Description("Internal item ID for update or delete.")),

			mcp.WithObject("attributes", mcp.Description("Values for approved shared fields. Title is optional. Links is a list of {name,url} references.")),
			mcp.WithString("primary_image_id", mcp.Description("Propose an existing photo on this item as primary.")),
			mcp.WithArray("remove_fields", mcp.Description("Explicit field keys to remove on update.")),
			mcp.WithArray("tags", mcp.Description("Overlapping organization labels, e.g. Games and Wii. Omit on update to preserve existing tags.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			attributes, err := json.Marshal(request.GetArguments()["attributes"])
			if err != nil || string(attributes) == "null" {
				attributes = []byte(`{}`)
			}
			var tags []string
			if raw, ok := request.GetArguments()["tags"]; ok {
				data, _ := json.Marshal(raw)
				if err := json.Unmarshal(data, &tags); err != nil {
					return mcp.NewToolResultError("tags must be an array of strings"), nil
				}
			}
			var location *string
			if value, ok := request.GetArguments()["location"].(string); ok {
				location = &value
			}
			var primary *string
			if value, ok := request.GetArguments()["primary_image_id"].(string); ok {
				primary = &value
			}
			var removeFields []string
			if raw, ok := request.GetArguments()["remove_fields"]; ok {
				data, _ := json.Marshal(raw)
				if err := json.Unmarshal(data, &removeFields); err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
			}
			proposal, err := remyService.ProposeItemChange(ctx, store.ItemProposalPayload{PrimaryImageID: primary, RemoveFields: removeFields, Tags: tags, Location: location,
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
		mcp.NewTool("revise_pending_proposal",
			mcp.WithDescription("Revise a pending proposal in place after the user asks for a correction. This does not approve or commit it."),
			mcp.WithString("proposal_id", mcp.Description("Pending proposal ID.")),
			mcp.WithObject("proposed_payload", mcp.Description("Complete revised category or item proposal payload.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			raw, err := json.Marshal(request.GetArguments()["proposed_payload"])
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			proposal, err := remyService.RevisePendingProposal(ctx, request.GetString("proposal_id", ""), raw)
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
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := remyService.QueryInventory(ctx, request.GetString("query", ""), "")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultStructuredOnly(result), nil
		},
	)

	server.AddTool(
		mcp.NewTool("list_items",
			mcp.WithDescription("Return a page of items, optionally matching all selected tags. Templates do not own items."),
			mcp.WithString("tags", mcp.Description("Comma-separated exact tags, all must match.")),
			mcp.WithNumber("limit", mcp.Description("Maximum number of items to return.")),
			mcp.WithNumber("offset", mcp.Description("Number of items to skip.")),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			items, _, err := repo.SearchLibrary(ctx, "", "", "", "", request.GetString("tags", ""), "", request.GetInt("limit", 50), request.GetInt("offset", 0))
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
