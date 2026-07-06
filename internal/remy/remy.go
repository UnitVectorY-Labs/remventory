package remy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/UnitVectorY-Labs/remventory/internal/config"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
)

type Service struct {
	cfg    config.Config
	store  *store.Store
	client *http.Client
}

type Request struct {
	Message string `json:"message"`
}

type Response struct {
	State      string      `json:"state"`
	Summary    string      `json:"summary"`
	Components []Component `json:"components"`
}

type Component struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type categoryDraft struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Attributes  []store.AttributeDraft `json:"attributes"`
}

type itemDraft struct {
	CategoryName string          `json:"category_name"`
	Title        string          `json:"title"`
	Attributes   json.RawMessage `json:"attributes"`
	Quantity     int             `json:"quantity"`
}

func New(cfg config.Config, repo *store.Store) *Service {
	return &Service{
		cfg:   cfg,
		store: repo,
		client: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (s *Service) Handle(ctx context.Context, req Request) (Response, error) {
	if s.store == nil {
		return Response{}, errors.New("database is not configured")
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		return Response{}, errors.New("message is required")
	}

	categories, err := s.store.ListCategories(ctx, 100, 0)
	if err != nil {
		return Response{}, err
	}

	lower := strings.ToLower(message)
	switch {
	case looksLikeCategoryRequest(lower, categories):
		return s.proposeCategory(ctx, message)
	case looksLikeListRequest(lower):
		return s.listItems(ctx, message, categories)
	case looksLikeItemAdd(lower):
		return s.proposeItem(ctx, message, categories)
	default:
		return Response{
			State:   "completed",
			Summary: "I can help create a tracking category, add an item, or list items in a category.",
			Components: []Component{{
				Type: "text",
				Data: "Try requests like \"I want to track my LEGO sets\", \"Add Super Mario Bros. Wonder for Nintendo Switch\", or \"Show me my video games.\"",
			}},
		}, nil
	}
}

func (s *Service) proposeCategory(ctx context.Context, message string) (Response, error) {
	draft, err := s.categoryDraft(ctx, message)
	if err != nil {
		return Response{}, err
	}

	proposal, err := s.store.CreateCategoryProposal(ctx, store.CategoryProposalPayload{
		Name:        draft.Name,
		Description: draft.Description,
		Attributes:  draft.Attributes,
	})
	if err != nil {
		return Response{}, err
	}

	return Response{
		State:   "proposing",
		Summary: "Review this category before it is added.",
		Components: []Component{{
			Type: "category_proposal",
			Data: proposal,
		}},
	}, nil
}

func (s *Service) proposeItem(ctx context.Context, message string, categories []store.Category) (Response, error) {
	if len(categories) == 0 {
		return Response{
			State:   "completed",
			Summary: "Create a category first so I know what kind of item to add.",
			Components: []Component{{
				Type: "text",
				Data: "For example: \"I want to track my video games.\"",
			}},
		}, nil
	}

	draft, err := s.itemDraft(ctx, message, categories)
	if err != nil {
		return Response{}, err
	}

	category := bestCategoryMatch(draft.CategoryName, categories)
	if category == nil {
		category = bestCategoryMatch(message, categories)
	}
	if category == nil {
		return Response{
			State:   "completed",
			Summary: "I could not confidently choose a category for that item.",
			Components: []Component{{
				Type: "category_list",
				Data: categories,
			}},
		}, nil
	}

	proposal, err := s.store.CreateItemProposal(ctx, store.ItemProposalPayload{
		Operation:  "create",
		CategoryID: category.ID,
		Title:      draft.Title,
		Attributes: draft.Attributes,
		Quantity:   draft.Quantity,
	})
	if err != nil {
		return Response{}, err
	}

	return Response{
		State:   "proposing",
		Summary: "Review this item before it is added.",
		Components: []Component{{
			Type: "item_proposal",
			Data: proposal,
		}},
	}, nil
}

func (s *Service) listItems(ctx context.Context, message string, categories []store.Category) (Response, error) {
	category := bestCategoryMatch(message, categories)
	if category == nil {
		return Response{
			State:   "completed",
			Summary: "Choose a category to list.",
			Components: []Component{{
				Type: "category_list",
				Data: categories,
			}},
		}, nil
	}

	items, err := s.store.ListItems(ctx, category.ID, 50, 0)
	if err != nil {
		return Response{}, err
	}

	return Response{
		State:   "completed",
		Summary: fmt.Sprintf("Found %d item(s) in %s.", len(items), category.Name),
		Components: []Component{{
			Type: "item_list",
			Data: map[string]any{
				"category": category,
				"items":    items,
			},
		}},
	}, nil
}

func (s *Service) categoryDraft(ctx context.Context, message string) (categoryDraft, error) {
	var draft categoryDraft
	if s.modelConfigured() {
		err := s.completeJSON(ctx, `Return only JSON for an inventory category proposal.
Schema:
{"name":"string","description":"string","attributes":[{"key":"snake_case","label":"string","data_type":"text|number|boolean|date","required":boolean,"display_order":number}]}
Use 4 to 7 practical attributes.`, message, &draft)
		if err == nil && draft.Name != "" && len(draft.Attributes) > 0 {
			normalizeAttributes(draft.Attributes)
			return draft, nil
		}
	}

	name := fallbackCategoryName(message)
	draft = categoryDraft{
		Name:        name,
		Description: "Items tracked in " + name + ".",
		Attributes: []store.AttributeDraft{
			{Key: "name", Label: "Name", DataType: "text", Required: true, DisplayOrder: 1},
			{Key: "status", Label: "Status", DataType: "text", DisplayOrder: 2},
			{Key: "notes", Label: "Notes", DataType: "text", DisplayOrder: 3},
		},
	}
	return draft, nil
}

func (s *Service) itemDraft(ctx context.Context, message string, categories []store.Category) (itemDraft, error) {
	var draft itemDraft
	if s.modelConfigured() {
		categoryBytes, _ := json.Marshal(categories)
		err := s.completeJSON(ctx, `Return only JSON for an inventory item proposal.
Schema:
{"category_name":"string","title":"string","attributes":{},"quantity":number}
Choose category_name from the provided category list. Put category-specific values in attributes using the category attribute keys.
Categories: `+string(categoryBytes), message, &draft)
		if err == nil && draft.Title != "" {
			if len(draft.Attributes) == 0 {
				draft.Attributes = json.RawMessage(`{}`)
			}
			if draft.Quantity == 0 {
				draft.Quantity = 1
			}
			return draft, nil
		}
	}

	return itemDraft{
		CategoryName: "",
		Title:        fallbackItemTitle(message),
		Attributes:   json.RawMessage(`{}`),
		Quantity:     1,
	}, nil
}

func (s *Service) completeJSON(ctx context.Context, system, user string, target any) error {
	body := map[string]any{
		"model": s.cfg.OpenAIModel,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.2,
		"stream":      false,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := strings.TrimRight(s.cfg.OpenAIBaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if s.cfg.OpenAIAPIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.cfg.OpenAIAPIKey)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("model endpoint returned %s", resp.Status)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}
	if len(parsed.Choices) == 0 {
		return errors.New("model returned no choices")
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return json.Unmarshal([]byte(strings.TrimSpace(content)), target)
}

func (s *Service) modelConfigured() bool {
	return s.cfg.OpenAIBaseURL != "" && s.cfg.OpenAIModel != ""
}

func looksLikeCategoryRequest(message string, categories []store.Category) bool {
	if strings.Contains(message, "track") || strings.Contains(message, "catalog") || strings.Contains(message, "category") {
		return true
	}
	return len(categories) == 0 && !looksLikeListRequest(message)
}

func looksLikeItemAdd(message string) bool {
	return strings.HasPrefix(message, "add ") || strings.Contains(message, " add ") || strings.Contains(message, "inventory ")
}

func looksLikeListRequest(message string) bool {
	return strings.HasPrefix(message, "show ") || strings.HasPrefix(message, "list ") || strings.Contains(message, "show me")
}

func bestCategoryMatch(text string, categories []store.Category) *store.Category {
	lower := strings.ToLower(text)
	var fallback *store.Category
	for i := range categories {
		name := strings.ToLower(categories[i].Name)
		if strings.Contains(lower, name) || strings.Contains(lower, strings.TrimSuffix(name, "s")) {
			return &categories[i]
		}
		if fallback == nil {
			fallback = &categories[i]
		}
	}
	if len(categories) == 1 {
		return fallback
	}
	return nil
}

func normalizeAttributes(attributes []store.AttributeDraft) {
	for i := range attributes {
		if attributes[i].DisplayOrder == 0 {
			attributes[i].DisplayOrder = i + 1
		}
		if attributes[i].DataType == "" {
			attributes[i].DataType = "text"
		}
		if len(attributes[i].Config) == 0 {
			attributes[i].Config = json.RawMessage(`{}`)
		}
	}
}

func fallbackCategoryName(message string) string {
	name := strings.TrimSpace(message)
	replacers := []string{
		"i want to track", "",
		"track my", "",
		"track", "",
		"catalog my", "",
		"catalog", "",
		".", "",
	}
	lower := strings.ToLower(name)
	for i := 0; i < len(replacers); i += 2 {
		lower = strings.ReplaceAll(lower, replacers[i], replacers[i+1])
	}
	name = strings.TrimSpace(lower)
	if name == "" {
		return "Inventory"
	}
	return strings.Title(name)
}

func fallbackItemTitle(message string) string {
	title := strings.TrimSpace(message)
	title = strings.TrimPrefix(strings.ToLower(title), "add ")
	if title == "" {
		return "Untitled item"
	}
	return strings.Title(title)
}
