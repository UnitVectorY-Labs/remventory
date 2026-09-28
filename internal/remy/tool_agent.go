package remy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/UnitVectorY-Labs/remventory/internal/store"
)

const remyAgentInstructions = `You are Remy, a playful, capable inventory assistant. Answer the person's actual question in natural language and choose useful typed interface components with present_evidence. You can search and aggregate the complete inventory with tools; ask tools for facts rather than guessing. Distinguish records (distinct items) from units (sum of quantity). Use stored item IDs for references and compare only items found by tools. Data changes must be proposed for explicit human approval. Never say a proposal was approved or inventory changed. Use propose_item_change or propose_category_change only when the user clearly requests a change. Ask a concise clarification instead of guessing an ambiguous target or missing quantity delta. Preserve fields on update. You have no approval tool. The browser focus, prior canonical references, and pending proposals below are refreshed from the server and support follow-up pronouns. The presentation library supports text, item_cards, item_list, item_detail, comparison, category_definition, category_list, statistic, grouped_statistic, and proposal cards. The same evidence may be presented in different supported forms; choose based on the request. Use only returned evidence IDs; presentation data is hydrated and validated by the server. Always provide a natural-language answer, even when using components.`

type evidence struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func (s *Service) agentToolDefinitions() []map[string]any {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	obj := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
	}
	fn := func(name, description string, parameters map[string]any) map[string]any {
		return map[string]any{"type": "function", "function": map[string]any{"name": name, "description": description, "parameters": parameters}}
	}
	return []map[string]any{
		fn("list_categories", "List a bounded page of inventory collections and their attribute schemas", obj(map[string]any{"offset": map[string]any{"type": "integer"}, "limit": map[string]any{"type": "integer"}})),
		fn("search_inventory", "Search inventory with server-side category and attribute filters. Returns exact total, a bounded page, and has_more.", obj(map[string]any{"query": str("Title or item text to search"), "category_id": str("Optional category UUID"), "attribute_key": str("Optional exact attribute key filter"), "attribute_value": str("Optional exact attribute value filter"), "offset": map[string]any{"type": "integer"}, "limit": map[string]any{"type": "integer"}})),
		fn("aggregate_inventory", "Compute deterministic counts, quantity totals, min/max, or grouped counts from all filtered inventory. metric is records, units, min, max, or group.", obj(map[string]any{"category_id": str("Optional category UUID"), "metric": str("records, units, min, max, or group"), "attribute_key": str("Attribute key for min/max/group"), "group_by": str("Attribute key for grouping"), "filter_key": str("Optional exact filter attribute key"), "filter_value": str("Optional exact filter value")}, "metric")),
		fn("compare_items", "Load canonical details for two or more exact item IDs", obj(map[string]any{"item_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "item_ids")),
		fn("get_category_definition", "Load one category schema by UUID", obj(map[string]any{"category_id": str("Category UUID")}, "category_id")),
		fn("propose_item_change", "Create a pending item proposal. Updates preserve omitted fields. quantity_adjust requires a nonzero explicit delta. This tool never commits.", obj(map[string]any{"operation": str("create, update, delete, or quantity_adjust"), "category_id": str("Category UUID"), "item_id": str("Existing item UUID for update/delete/quantity_adjust"), "title": str("Item title, required for create"), "attributes": map[string]any{"type": "object", "additionalProperties": true}, "quantity": map[string]any{"type": "integer"}, "quantity_delta": map[string]any{"type": "integer"}}, "operation", "category_id")),
		fn("propose_category_change", "Create a pending category proposal for explicit human review", obj(map[string]any{"operation": str("create, update, delete"), "category_id": str("Existing category UUID"), "name": str("Category name"), "description": str("Category description"), "attributes": map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": true}}}, "operation")),
		fn("revise_pending_proposal", "Revise an existing pending proposal in place after the user explicitly requests a correction. Supply a complete valid payload and preserve all omitted fields.", obj(map[string]any{"proposal_id": str("Pending proposal UUID"), "proposed_payload": map[string]any{"type": "object", "additionalProperties": true}}, "proposal_id", "proposed_payload")),
		fn("present_evidence", "Choose a typed presentation and evidence ID returned by a prior tool. The same inventory evidence can be shown as cards, a compact list, a detail view, or a comparison. No arbitrary display data is accepted.", obj(map[string]any{"evidence_id": str("Evidence identifier returned by an inventory tool"), "component_type": str("text, item_cards, item_list, item_detail, comparison, category_definition, category_list, statistic, grouped_statistic"), "title": str("Short display title")}, "evidence_id", "component_type")),
	}
}

func (s *Service) executeAgentTool(ctx context.Context, name string, args map[string]any, evidenceByID *map[string]evidence, seq *int) (any, *Component, error) {
	register := func(typ string, data any) string {
		*seq++
		id := fmt.Sprintf("evidence-%d", *seq)
		(*evidenceByID)[id] = evidence{Type: typ, Data: data}
		return id
	}
	str := func(k string) string { v, _ := args[k].(string); return strings.TrimSpace(v) }
	switch name {
	case "list_categories":
		offset, limit := intArg(args, "offset", 0), intArg(args, "limit", 50)
		if limit < 1 || limit > 100 {
			limit = 50
		}
		if offset < 0 {
			offset = 0
		}
		page, err := s.store.ListCategories(ctx, limit+1, offset)
		if err != nil {
			return nil, nil, err
		}
		hasMore := len(page) > limit
		if hasMore {
			page = page[:limit]
		}
		id := register("categories", page)
		return map[string]any{"categories": page, "offset": offset, "limit": limit, "has_more": hasMore, "evidence_id": id}, nil, nil
	case "search_inventory":
		categoryID := str("category_id")
		query := str("query")
		key, value := str("attribute_key"), str("attribute_value")
		offset, limit := intArg(args, "offset", 0), intArg(args, "limit", 50)
		search, err := s.Search(ctx, query, categoryID, key, value, limit, offset)
		if err != nil {
			return nil, nil, err
		}
		data := map[string]any{"judgment": "matches", "summary": fmt.Sprintf("Found %d matching records.", search.Total), "categories": search.Categories, "matches": search.Items, "total_records": search.Total, "offset": search.Offset, "limit": search.Limit, "has_more": search.HasMore}
		id := register("query_result", data)
		return map[string]any{"total_records": search.Total, "offset": search.Offset, "limit": search.Limit, "has_more": search.HasMore, "items": search.Items, "categories": search.Categories, "evidence_id": id}, nil, nil
	case "aggregate_inventory":
		catID, metric := str("category_id"), str("metric")
		fk, fv, key := str("filter_key"), str("filter_value"), str("attribute_key")
		result, err := s.Aggregate(ctx, catID, metric, key, str("group_by"), fk, fv)
		if err != nil {
			return nil, nil, err
		}
		label, value, detail := metric, "", ""
		switch metric {
		case "records":
			value = fmt.Sprint(result["records"])
			label = "Records"
		case "units":
			value = fmt.Sprint(result["units"])
			label = "Units"
			detail = fmt.Sprint(result["records"], " distinct records")
		case "group":
			label = "Items by " + str("group_by")
		case "min", "max":
			label = str("attribute_key") + " " + metric
			value = fmt.Sprint(result[metric])
		}
		result["label"], result["value"], result["detail"] = label, value, detail
		id := register("statistic", result)
		result["evidence_id"] = id
		return result, nil, nil
	case "compare_items":
		ids, _ := args["item_ids"].([]any)
		if len(ids) < 2 || len(ids) > 10 {
			return nil, nil, errors.New("comparison needs between two and ten item IDs")
		}
		data := []map[string]any{}
		for _, raw := range ids {
			id, ok := raw.(string)
			if !ok {
				return nil, nil, errors.New("invalid item ID")
			}
			item, e := s.store.GetItem(ctx, id)
			if e != nil {
				return nil, nil, e
			}
			cat, e := s.store.GetCategoryDefinition(ctx, item.CategoryID)
			if e != nil {
				return nil, nil, e
			}
			data = append(data, map[string]any{"item": item, "category": cat})
		}
		eid := register("comparison", data)
		return map[string]any{"items": data, "evidence_id": eid}, nil, nil
	case "get_category_definition":
		cat, err := s.store.GetCategoryDefinition(ctx, str("category_id"))
		if err != nil {
			return nil, nil, err
		}
		id := register("category_definition", cat)
		return map[string]any{"category": cat, "evidence_id": id}, nil, nil
	case "propose_item_change":
		op, catID, itemID := str("operation"), str("category_id"), str("item_id")
		title := str("title")
		rawAttrs, _ := json.Marshal(args["attributes"])
		if string(rawAttrs) == "null" {
			rawAttrs = []byte(`{}`)
		}
		quantity := intArg(args, "quantity", 1)
		delta := intArg(args, "quantity_delta", 0)
		if op == "" {
			return nil, nil, errors.New("operation is required")
		}
		if op == "update" {
			if raw, ok := args["attributes"]; ok {
				if _, valid := raw.(map[string]any); !valid {
					return nil, nil, errors.New("item attributes must be a JSON object")
				}
			}
		}
		p, err := s.ProposeItemChange(ctx, store.ItemProposalPayload{Operation: op, CategoryID: catID, ItemID: itemID, Title: title, Attributes: rawAttrs, Quantity: quantity, QuantityDelta: delta})
		if err != nil {
			return nil, nil, err
		}
		comp := Component{Type: "item_proposal", Data: p}
		return map[string]any{"proposal": p, "presented": true}, &comp, nil
	case "propose_category_change":
		operation, categoryID := str("operation"), str("category_id")
		attrs := []store.AttributeDraft{}
		if raw, ok := args["attributes"]; ok {
			b, _ := json.Marshal(raw)
			if len(b) == 0 || b[0] != '[' {
				return nil, nil, errors.New("attributes must be a valid array of category attribute definitions")
			}
			if err := json.Unmarshal(b, &attrs); err != nil {
				return nil, nil, errors.New("attributes must be a valid array of category attribute definitions")
			}
		} else if operation == "update" {
			current, e := s.store.GetCategoryDefinition(ctx, categoryID)
			if e != nil {
				return nil, nil, e
			}
			for _, a := range current.Attributes {
				attrs = append(attrs, store.AttributeDraft{Key: a.Key, Label: a.Label, DataType: a.DataType, Required: a.Required, DisplayOrder: a.DisplayOrder, Config: a.Config})
			}
			if str("name") == "" {
				args["name"] = current.Name
			}
			if _, ok := args["description"]; !ok {
				args["description"] = current.Description
			}
		}
		p, err := s.ProposeCategoryChange(ctx, store.CategoryProposalPayload{Operation: operation, CategoryID: categoryID, Name: str("name"), Description: str("description"), Attributes: attrs})
		if err != nil {
			return nil, nil, err
		}
		comp := Component{Type: "category_proposal", Data: p}
		return map[string]any{"proposal": p, "presented": true}, &comp, nil
	case "revise_pending_proposal":
		payload, ok := args["proposed_payload"]
		if !ok {
			return nil, nil, errors.New("complete proposed_payload is required")
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, err
		}
		proposal, err := s.RevisePendingProposal(ctx, str("proposal_id"), encoded)
		if err != nil {
			return nil, nil, err
		}
		componentType := "item_proposal"
		if proposal.Type == "category_create" {
			componentType = "category_proposal"
		}
		comp := Component{Type: componentType, Data: proposal}
		return map[string]any{"proposal": proposal, "presented": true}, &comp, nil
	case "present_evidence":
		e, ok := (*evidenceByID)[str("evidence_id")]
		if !ok {
			return nil, nil, errors.New("evidence ID was not returned by an inventory tool")
		}
		componentType := str("component_type")
		if !allowedPresentation(componentType) || !compatiblePresentation(e.Type, componentType) {
			return nil, nil, errors.New("unsupported presentation component")
		}
		data := e.Data
		if e.Type == "proposal" {
			if c, ok := data.(Component); ok {
				return map[string]any{"presented": true}, &c, nil
			}
		}
		switch componentType {
		case "category_list":
			data = map[string]any{"categories": data}
		case "item_cards":
			if result, ok := data.(map[string]any); ok {
				data = map[string]any{"matches": result["matches"], "categories": result["categories"], "title": str("title")}
			}
		case "item_list":
			if result, ok := data.(map[string]any); ok {
				cats, _ := result["categories"].([]store.Category)
				cat := store.Category{}
				if len(cats) == 1 {
					cat = cats[0]
				}
				data = map[string]any{"category": cat, "items": result["matches"], "total_records": result["total_records"], "has_more": result["has_more"]}
			}
		case "item_detail":
			if result, ok := data.(map[string]any); ok {
				items, _ := result["matches"].([]store.Item)
				cats, _ := result["categories"].([]store.Category)
				if len(items) == 0 {
					return nil, nil, errors.New("no matching item to show")
				}
				var cat store.Category
				for _, c := range cats {
					if c.ID == items[0].CategoryID {
						cat = c
						break
					}
				}
				data = map[string]any{"item": items[0], "category": cat}
			}
		case "comparison":
			if result, ok := data.(map[string]any); ok {
				data = map[string]any{"title": str("title"), "rows": comparisonRows(result), "item_ids": comparisonItemIDs(result)}
			} else {
				data = map[string]any{"title": str("title"), "rows": comparisonRowsFrom(data), "item_ids": comparisonItemIDsFrom(data)}
			}
		case "statistic", "grouped_statistic":
			if result, ok := data.(map[string]any); ok {
				if str("title") != "" {
					result["label"] = str("title")
				}
				data = result
			}
		}
		component := Component{Type: componentType, Data: data}
		if componentType == "text" {
			component.Data = map[string]any{"text": str("title")}
		}
		return map[string]any{"presented": true, "type": componentType}, &component, nil
	default:
		return nil, nil, fmt.Errorf("unknown tool %q", name)
	}
}

func allowedPresentation(s string) bool {
	switch s {
	case "text", "item_cards", "item_list", "item_detail", "comparison", "category_definition", "category_list", "statistic", "grouped_statistic", "proposal":
		return true
	}
	return false
}

func compatiblePresentation(evidenceType, componentType string) bool {
	switch evidenceType {
	case "query_result":
		return componentType == "item_cards" || componentType == "item_list" || componentType == "item_detail" || componentType == "comparison" || componentType == "text"
	case "items":
		return componentType == "item_list" || componentType == "item_detail" || componentType == "comparison"
	case "category_definition":
		return componentType == "category_definition" || componentType == "text"
	case "categories":
		return componentType == "category_list" || componentType == "text"
	case "statistic":
		return componentType == "statistic" || componentType == "grouped_statistic" || componentType == "text"
	case "proposal":
		return componentType == "proposal"
	case "comparison":
		return componentType == "comparison"
	default:
		return false
	}
}

func comparisonRows(result map[string]any) [][]any {
	items, _ := result["matches"].([]store.Item)
	if len(items) == 0 {
		if raw, ok := result["items"].([]map[string]any); ok {
			for _, entry := range raw {
				if item, ok := entry["item"].(store.Item); ok {
					items = append(items, item)
				}
			}
		}
	}
	rows := [][]any{{"Item", "Quantity", "Details"}}
	for _, item := range items {
		rows = append(rows, []any{item.Title, item.Quantity, string(item.Attributes)})
	}
	return rows
}

func comparisonRowsFrom(value any) [][]any {
	rows := [][]any{{"Item", "Quantity", "Details"}}
	entries, ok := value.([]map[string]any)
	if !ok {
		return rows
	}
	for _, entry := range entries {
		item, ok := entry["item"].(store.Item)
		if !ok {
			continue
		}
		rows = append(rows, []any{item.Title, item.Quantity, string(item.Attributes)})
	}
	return rows
}

func comparisonItemIDs(result map[string]any) []string {
	items, _ := result["matches"].([]store.Item)
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func comparisonItemIDsFrom(value any) []string {
	entries, ok := value.([]map[string]any)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if item, ok := entry["item"].(store.Item); ok {
			ids = append(ids, item.ID)
		}
	}
	return ids
}
func intArg(args map[string]any, key string, fallback int) int {
	v, ok := args[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return fallback
}
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case string:
		var f float64
		_, err := fmt.Sscan(n, &f)
		return f, err == nil
	}
	return 0, false
}

func (s *Service) hydrateFocus(ctx context.Context, ids []string) ([]any, error) {
	out := []any{}
	for _, id := range ids {
		if p, err := s.store.GetProposal(ctx, id); err == nil && p.Status == "pending" {
			out = append(out, p)
			continue
		}
		if item, err := s.store.GetItem(ctx, id); err == nil {
			cat, e := s.store.GetCategoryDefinition(ctx, item.CategoryID)
			if e != nil {
				return nil, e
			}
			out = append(out, map[string]any{"item": item, "category": cat})
			continue
		}
		if cat, err := s.store.GetCategoryDefinition(ctx, id); err == nil {
			out = append(out, cat)
		}
	}
	return out, nil
}
