package remy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/UnitVectorY-Labs/remventory/internal/store"
)

// SearchResult is the shared bounded inventory search capability used by Remy and MCP.
type SearchResult struct {
	Items      []store.Item     `json:"items"`
	Categories []store.Category `json:"categories"`
	Total      int              `json:"total_records"`
	Offset     int              `json:"offset"`
	Limit      int              `json:"limit"`
	HasMore    bool             `json:"has_more"`
}

func (s *Service) Search(ctx context.Context, query, categoryID, attributeKey, attributeValue string, limit, offset int) (SearchResult, error) {
	items, total, err := s.store.SearchInventory(ctx, query, categoryID, attributeKey, attributeValue, limit, offset)
	if err != nil {
		return SearchResult{}, err
	}
	categories := []store.Category{}
	seen := map[string]bool{}
	for _, item := range items {
		if !seen[item.CategoryID] {
			category, e := s.store.GetCategoryDefinition(ctx, item.CategoryID)
			if e != nil {
				return SearchResult{}, e
			}
			categories = append(categories, category)
			seen[item.CategoryID] = true
		}
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return SearchResult{Items: items, Categories: categories, Total: total, Offset: offset, Limit: limit, HasMore: offset+len(items) < total}, nil
}

func (s *Service) Aggregate(ctx context.Context, categoryID, metric, attributeKey, groupBy, filterKey, filterValue string) (map[string]any, error) {
	var result map[string]any
	var err error
	if metric == "group" {
		var groups map[string]int
		groups, err = s.store.GroupInventory(ctx, categoryID, groupBy, filterKey, filterValue)
		result = map[string]any{"groups": groups}
	} else {
		result, err = s.store.AggregateInventory(ctx, categoryID, metric, attributeKey, filterKey, filterValue)
	}
	if err != nil {
		return nil, err
	}
	label, value, detail := metric, "", ""
	switch metric {
	case "records":
		label = "Records"
		value = fmt.Sprint(result["records"])
	case "units":
		label = "Units"
		value = fmt.Sprint(result["units"])
		detail = fmt.Sprint(result["records"], " distinct records")
	case "group":
		label = "Items by " + groupBy
	case "min", "max":
		label = attributeKey + " " + metric
		value = fmt.Sprint(result[metric])
	}
	result["label"], result["value"], result["detail"] = label, value, detail
	return result, nil
}

// ProposeItemChange validates shared mutation semantics and always returns a pending proposal.
func (s *Service) ProposeItemChange(ctx context.Context, payload store.ItemProposalPayload) (store.Proposal, error) {
	if payload.Operation == "quantity_adjust" && payload.QuantityDelta == 0 {
		return store.Proposal{}, errors.New("quantity adjustment needs an explicit nonzero quantity_delta")
	}
	if payload.Operation == "create" {
		matches, _, err := s.store.SearchInventory(ctx, payload.Title, payload.CategoryID, "", "", 100, 0)
		if err != nil {
			return store.Proposal{}, err
		}
		for _, item := range matches {
			if strings.EqualFold(strings.TrimSpace(item.Title), strings.TrimSpace(payload.Title)) {
				return store.Proposal{}, fmt.Errorf("an item named %q already exists in this category; identify it and propose a quantity adjustment, or clarify how the new item differs", payload.Title)
			}
		}
	} else if payload.Operation == "update" || payload.Operation == "delete" || payload.Operation == "quantity_adjust" {
		item, err := s.store.GetItem(ctx, payload.ItemID)
		if err != nil {
			return store.Proposal{}, errors.New("identify the exact item before proposing a change")
		}
		if item.CategoryID != payload.CategoryID {
			return store.Proposal{}, errors.New("item does not belong to the supplied category")
		}
		if payload.Title == "" {
			payload.Title = item.Title
		}
		if payload.Operation == "update" {
			old := map[string]any{}
			next := map[string]any{}
			if err := json.Unmarshal(item.Attributes, &old); err != nil {
				return store.Proposal{}, err
			}
			if len(payload.Attributes) > 0 {
				if err := json.Unmarshal(payload.Attributes, &next); err != nil {
					return store.Proposal{}, errors.New("item attributes must be a JSON object")
				}
			}
			for key, value := range next {
				old[key] = value
			}
			payload.Attributes, _ = json.Marshal(old)
			if payload.Quantity == 0 {
				payload.Quantity = item.Quantity
			}
		}
		if payload.Operation == "delete" || payload.Operation == "quantity_adjust" {
			payload.Attributes = item.Attributes
			payload.Quantity = item.Quantity
		}
	}
	return s.store.CreateItemProposal(ctx, payload)
}

func (s *Service) ProposeCategoryChange(ctx context.Context, payload store.CategoryProposalPayload) (store.Proposal, error) {
	if payload.Operation == "update" || payload.Operation == "delete" {
		category, err := s.store.GetCategoryDefinition(ctx, payload.CategoryID)
		if err != nil {
			return store.Proposal{}, err
		}
		if strings.TrimSpace(payload.Name) == "" {
			payload.Name = category.Name
		}
		if strings.TrimSpace(payload.Description) == "" {
			payload.Description = category.Description
		}
		if len(payload.Attributes) == 0 {
			payload.Attributes = make([]store.AttributeDraft, 0, len(category.Attributes))
			for _, a := range category.Attributes {
				payload.Attributes = append(payload.Attributes, store.AttributeDraft{Key: a.Key, Label: a.Label, DataType: a.DataType, Required: a.Required, DisplayOrder: a.DisplayOrder, Config: a.Config})
			}
		}
	}
	return s.store.CreateCategoryProposal(ctx, payload)
}

func (s *Service) RevisePendingProposal(ctx context.Context, id string, payload json.RawMessage) (store.Proposal, error) {
	return s.store.RevisePendingProposal(ctx, id, payload)
}
