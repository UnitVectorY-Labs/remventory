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

	"github.com/UnitVectorY-Labs/remventory/internal/agentruntime"
	"github.com/UnitVectorY-Labs/remventory/internal/agui"
	"github.com/UnitVectorY-Labs/remventory/internal/config"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
	"google.golang.org/adk/agent"
)

type Service struct {
	cfg    config.Config
	store  *store.Store
	client *http.Client
	agent  agent.Agent
}

type Request struct {
	Message string          `json:"message"`
	Context *VisibleContext `json:"context,omitempty"`
}

type Response struct {
	State          string       `json:"state"`
	Summary        string       `json:"summary"`
	RequestSummary string       `json:"request_summary,omitempty"`
	Components     []Component  `json:"components"`
	Events         []agui.Event `json:"events,omitempty"`
}

type VisibleContext struct {
	State          string      `json:"state,omitempty"`
	Summary        string      `json:"summary,omitempty"`
	RequestSummary string      `json:"request_summary,omitempty"`
	Components     []Component `json:"components,omitempty"`
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

type QueryResult struct {
	Judgment   string         `json:"judgment"`
	Confidence string         `json:"confidence,omitempty"`
	Summary    string         `json:"summary"`
	Category   store.Category `json:"category"`
	Matches    []store.Item   `json:"matches"`
}

type modelQueryResult struct {
	Judgment       string   `json:"judgment"`
	Confidence     string   `json:"confidence"`
	Summary        string   `json:"summary"`
	MatchedItemIDs []string `json:"matched_item_ids"`
}

type requestPlan struct {
	Action string `json:"action"`
}

type categorySelection struct {
	CategoryID string `json:"category_id"`
}

type proposalRevision struct {
	ProposalID      string          `json:"proposal_id"`
	Summary         string          `json:"summary"`
	ProposedPayload json.RawMessage `json:"proposed_payload"`
}

type contextualAnswer struct {
	Answer string `json:"answer"`
}

type requestSummary struct {
	Summary string `json:"summary"`
}

func New(cfg config.Config, repo *store.Store) *Service {
	remyAgent, _ := agentruntime.NewRemyAgent(cfg)
	return &Service{
		cfg:   cfg,
		store: repo,
		client: &http.Client{
			Timeout: 45 * time.Second,
		},
		agent: remyAgent,
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

	action, err := s.planRequest(ctx, message, categories, req.Context)
	if err != nil {
		return Response{}, err
	}
	var response Response
	switch action {
	case "category_change":
		response, err = s.proposeCategory(ctx, message)
	case "query_inventory":
		response, err = s.queryInventory(ctx, message, categories)
	case "list_items":
		response, err = s.listItems(ctx, message, categories)
	case "item_change":
		response, err = s.proposeItem(ctx, message, categories)
	case "revise_proposal":
		response, err = s.reviseProposal(ctx, message, req.Context)
	case "answer_context":
		response, err = s.answerContext(ctx, message, req.Context)
	default:
		response = withEvents(Response{
			State:   "completed",
			Summary: "I can help create a tracking category, add an item, or list items in a category.",
			Components: []Component{{
				Type: "text",
				Data: "Try requests like \"I want to track my LEGO sets\", \"Add Super Mario Bros. Wonder for Nintendo Switch\", or \"Show me my video games.\"",
			}},
		})
	}
	if err != nil {
		return Response{}, err
	}
	response.RequestSummary = s.summarizeRequest(ctx, message)
	return response, nil
}

func (s *Service) planRequest(ctx context.Context, message string, categories []store.Category, visible *VisibleContext) (string, error) {
	if s.modelConfigured(s.cfg.MainModel) {
		categoryBytes, _ := json.Marshal(categories)
		contextBytes, _ := json.Marshal(visible)
		var plan requestPlan
		err := s.completeJSON(ctx, s.cfg.MainModel, `Classify the inventory request and return only JSON.
Schema: {"action":"category_change|item_change|list_items|query_inventory|revise_proposal|answer_context|help"}
Use category_change when the user wants to start tracking a kind of thing or change which attributes are tracked.
Use item_change when the user wants to add, update, remove, or change the quantity of an inventory item.
Use list_items when the user wants to browse or see inventory.
Use query_inventory when the user asks whether a particular item is owned or present.
Use revise_proposal when the visible context contains a pending proposal and the user asks to change or correct it.
Use answer_context when the user asks a question about the visible result or proposal without requesting a data change.
Available category definitions: `+string(categoryBytes)+`
Currently visible interface: `+string(contextBytes), message, &plan)
		if err != nil {
			return "", fmt.Errorf("ask model to interpret request: %w", err)
		}
		switch plan.Action {
		case "category_change", "item_change", "list_items", "query_inventory", "revise_proposal", "answer_context", "help":
			return plan.Action, nil
		default:
			return "", fmt.Errorf("model returned unsupported action %q", plan.Action)
		}
	}

	lower := strings.ToLower(message)
	switch {
	case looksLikeExistenceQuery(lower):
		return "query_inventory", nil
	case looksLikeListRequest(lower):
		return "list_items", nil
	case looksLikeItemAdd(lower):
		return "item_change", nil
	case looksLikeCategoryRequest(lower, categories):
		return "category_change", nil
	default:
		return "help", nil
	}
}

func (s *Service) reviseProposal(ctx context.Context, message string, visible *VisibleContext) (Response, error) {
	proposal, componentType, err := pendingProposalFromContext(visible)
	if err != nil {
		return Response{}, err
	}
	stored, err := s.store.GetProposal(ctx, proposal.ID)
	if err != nil {
		return Response{}, err
	}
	if stored.Status != "pending" {
		return Response{}, errors.New("the proposal on screen is no longer pending")
	}

	var revision proposalRevision
	err = s.completeJSON(ctx, s.cfg.ThinkingModel, `Revise the pending inventory proposal according to the user's follow-up request.
Return only JSON with schema:
{"proposal_id":"string","summary":"brief description of what changed","proposed_payload":{}}
proposal_id must remain unchanged. proposed_payload must be the complete revised payload, preserving every unchanged field and using the same schema as the current payload.
Current proposal: `+mustJSON(stored), message, &revision)
	if err != nil {
		return Response{}, fmt.Errorf("ask model to revise proposal: %w", err)
	}
	if revision.ProposalID != stored.ID {
		return Response{}, errors.New("model did not preserve the proposal id")
	}
	revised, err := s.store.RevisePendingProposal(ctx, stored.ID, revision.ProposedPayload)
	if err != nil {
		return Response{}, err
	}
	if revision.Summary == "" {
		revision.Summary = "I revised the proposal. Review it before approving."
	}
	return withEvents(Response{State: "proposing", Summary: revision.Summary, Components: []Component{{Type: componentType, Data: revised}}}), nil
}

func (s *Service) answerContext(ctx context.Context, message string, visible *VisibleContext) (Response, error) {
	if visible == nil || len(visible.Components) == 0 {
		return Response{}, errors.New("there is no visible result to answer a question about")
	}
	var answer contextualAnswer
	err := s.completeJSON(ctx, s.cfg.ThinkingModel, `Answer the user's question using only the currently visible Remventory interface data.
Return only JSON with schema {"answer":"concise answer"}. Do not claim that data changed and do not create a proposal.
Visible interface: `+mustJSON(visible), message, &answer)
	if err != nil {
		return Response{}, fmt.Errorf("ask model about visible context: %w", err)
	}
	components := make([]Component, 0, len(visible.Components)+1)
	components = append(components, Component{Type: "text", Data: answer.Answer})
	components = append(components, visible.Components...)
	state := visible.State
	if state == "" {
		state = "completed"
	}
	return withEvents(Response{State: state, Summary: answer.Answer, Components: components}), nil
}

func pendingProposalFromContext(visible *VisibleContext) (store.Proposal, string, error) {
	if visible != nil {
		for _, component := range visible.Components {
			if component.Type != "category_proposal" && component.Type != "item_proposal" {
				continue
			}
			var proposal store.Proposal
			raw, _ := json.Marshal(component.Data)
			if json.Unmarshal(raw, &proposal) == nil && proposal.ID != "" && proposal.Status == "pending" {
				return proposal, component.Type, nil
			}
		}
	}
	return store.Proposal{}, "", errors.New("there is no pending proposal on screen to revise")
}

func (s *Service) summarizeRequest(ctx context.Context, message string) string {
	if !s.modelConfigured(s.cfg.TinyModel) {
		return truncate(message, 80)
	}
	var summary requestSummary
	err := s.completeJSON(ctx, s.cfg.TinyModel, `Summarize the user's request as a neutral interface label of at most 10 words. Return only JSON: {"summary":"string"}.`, message, &summary)
	if err != nil || strings.TrimSpace(summary.Summary) == "" {
		return truncate(message, 80)
	}
	return summary.Summary
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

	return withEvents(Response{
		State:   "proposing",
		Summary: "Review this category before it is added.",
		Components: []Component{{
			Type: "category_proposal",
			Data: proposal,
		}},
	}), nil
}

func (s *Service) proposeItem(ctx context.Context, message string, categories []store.Category) (Response, error) {
	if len(categories) == 0 {
		return withEvents(Response{
			State:   "completed",
			Summary: "Create a category first so I know what kind of item to add.",
			Components: []Component{{
				Type: "text",
				Data: "For example: \"I want to track my video games.\"",
			}},
		}), nil
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
		return withEvents(Response{
			State:   "completed",
			Summary: "I could not confidently choose a category for that item.",
			Components: []Component{{
				Type: "category_list",
				Data: categories,
			}},
		}), nil
	}

	items, err := s.store.ListItems(ctx, category.ID, 200, 0)
	if err != nil {
		return Response{}, err
	}
	duplicate, err := s.matchExistingItem(ctx, message, *category, items, draft.Title)
	if err != nil {
		return Response{}, err
	}
	if duplicate.Judgment == "yes" && len(duplicate.Matches) > 0 {
		proposal, err := s.store.CreateItemProposal(ctx, store.ItemProposalPayload{
			Operation:     "quantity_adjust",
			CategoryID:    category.ID,
			ItemID:        duplicate.Matches[0].ID,
			Title:         duplicate.Matches[0].Title,
			Attributes:    duplicate.Matches[0].Attributes,
			QuantityDelta: maxInt(draft.Quantity, 1),
		})
		if err != nil {
			return Response{}, err
		}
		return withEvents(Response{
			State:   "proposing",
			Summary: "This looks like something already in inventory. Review the quantity change before it is committed.",
			Components: []Component{
				{Type: "query_result", Data: duplicate},
				{Type: "item_proposal", Data: proposal},
			},
		}), nil
	}
	if duplicate.Judgment == "uncertain" && len(duplicate.Matches) > 0 {
		return withEvents(Response{
			State:   "completed",
			Summary: "I found possible matches. Review them before deciding whether this should be added as a new item.",
			Components: []Component{{
				Type: "query_result",
				Data: duplicate,
			}},
		}), nil
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

	return withEvents(Response{
		State:   "proposing",
		Summary: "Review this item before it is added.",
		Components: []Component{{
			Type: "item_proposal",
			Data: proposal,
		}},
	}), nil
}

func (s *Service) listItems(ctx context.Context, message string, categories []store.Category) (Response, error) {
	category := bestCategoryMatch(message, categories)
	if category == nil {
		return withEvents(Response{
			State:   "completed",
			Summary: "Choose a category to list.",
			Components: []Component{{
				Type: "category_list",
				Data: categories,
			}},
		}), nil
	}

	items, err := s.store.ListItems(ctx, category.ID, 50, 0)
	if err != nil {
		return Response{}, err
	}

	return withEvents(Response{
		State:   "completed",
		Summary: fmt.Sprintf("Found %d item(s) in %s.", len(items), category.Name),
		Components: []Component{{
			Type: "item_list",
			Data: map[string]any{
				"category": category,
				"items":    items,
			},
		}},
	}), nil
}

func (s *Service) QueryInventory(ctx context.Context, message string, categoryID string) (QueryResult, error) {
	categories, err := s.store.ListCategories(ctx, 100, 0)
	if err != nil {
		return QueryResult{}, err
	}

	var category *store.Category
	if categoryID != "" {
		for i := range categories {
			if categories[i].ID == categoryID {
				category = &categories[i]
				break
			}
		}
	} else {
		category, err = s.selectCategory(ctx, message, categories)
		if err != nil {
			return QueryResult{}, err
		}
	}
	if category == nil {
		return QueryResult{
			Judgment: "uncertain",
			Summary:  "I could not confidently choose a category for this query.",
		}, nil
	}

	items, err := s.store.ListItems(ctx, category.ID, 500, 0)
	if err != nil {
		return QueryResult{}, err
	}
	return s.matchExistingItem(ctx, message, *category, items, message)
}

func (s *Service) queryInventory(ctx context.Context, message string, categories []store.Category) (Response, error) {
	category, err := s.selectCategory(ctx, message, categories)
	if err != nil {
		return Response{}, err
	}
	categoryID := ""
	if category != nil {
		categoryID = category.ID
	}

	result, err := s.QueryInventory(ctx, message, categoryID)
	if err != nil {
		return Response{}, err
	}

	return withEvents(Response{
		State:   "completed",
		Summary: result.Summary,
		Components: []Component{{
			Type: "query_result",
			Data: result,
		}},
	}), nil
}

func (s *Service) selectCategory(ctx context.Context, message string, categories []store.Category) (*store.Category, error) {
	if len(categories) == 0 {
		return nil, nil
	}
	if category := bestCategoryMatch(message, categories); category != nil {
		return category, nil
	}
	if !s.modelConfigured(s.cfg.MainModel) {
		return nil, nil
	}

	categoryBytes, _ := json.Marshal(categories)
	var selection categorySelection
	err := s.completeJSON(ctx, s.cfg.MainModel, `Choose the single most relevant inventory category for the request and return only JSON.
Schema: {"category_id":"uuid or empty string"}
Use only an id from the provided categories. Return an empty category_id when no category is reasonably relevant.
Categories: `+string(categoryBytes), message, &selection)
	if err != nil {
		return nil, fmt.Errorf("ask model to choose category: %w", err)
	}
	for i := range categories {
		if categories[i].ID == selection.CategoryID {
			return &categories[i], nil
		}
	}
	return nil, nil
}

func (s *Service) categoryDraft(ctx context.Context, message string) (categoryDraft, error) {
	var draft categoryDraft
	if s.modelConfigured(s.cfg.MainModel) {
		err := s.completeJSON(ctx, s.cfg.MainModel, `Return only JSON for an inventory category proposal.
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
	if s.modelConfigured(s.cfg.MainModel) {
		categoryBytes, _ := json.Marshal(categories)
		err := s.completeJSON(ctx, s.cfg.MainModel, `Return only JSON for an inventory item proposal.
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

func (s *Service) matchExistingItem(ctx context.Context, message string, category store.Category, items []store.Item, title string) (QueryResult, error) {
	if len(items) == 0 {
		return QueryResult{
			Judgment: "no",
			Summary:  "No matching items are in this category yet.",
			Category: category,
			Matches:  nil,
		}, nil
	}

	if s.modelConfigured(s.cfg.ThinkingModel) {
		var modelResult modelQueryResult
		itemBytes, _ := json.Marshal(items)
		err := s.completeJSON(ctx, s.cfg.ThinkingModel, `Return only JSON for an inventory existence check.
Schema:
{"judgment":"yes|no|uncertain","confidence":"low|medium|high","summary":"string","matched_item_ids":["string"]}
Use yes only when the inventory clearly contains the requested item. Use uncertain for likely variants or partial matches.
Inventory items: `+string(itemBytes), message, &modelResult)
		if err == nil && validJudgment(modelResult.Judgment) {
			return queryResultFromModel(modelResult, category, items), nil
		}
	}

	matches := fallbackMatches(message+" "+title, items)
	judgment := "no"
	summary := "No matching item appears to be in this category."
	confidence := "medium"
	if len(matches) > 0 {
		judgment = "uncertain"
		summary = "Found possible matching item(s)."
		confidence = "low"
		if normalizedTitle(matches[0].Title) == normalizedTitle(title) {
			judgment = "yes"
			summary = "Found a matching item already in inventory."
			confidence = "medium"
		}
	}

	return QueryResult{
		Judgment:   judgment,
		Confidence: confidence,
		Summary:    summary,
		Category:   category,
		Matches:    matches,
	}, nil
}

func (s *Service) completeJSON(ctx context.Context, model, system, user string, target any) error {
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.2,
		"stream":      false,
		"response_format": map[string]string{
			"type": "json_object",
		},
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

func (s *Service) modelConfigured(model string) bool {
	return s.cfg.OpenAIBaseURL != "" && model != ""
}

func mustJSON(value any) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit-1]) + "…"
}

func withEvents(response Response) Response {
	components := make([]agui.Component, 0, len(response.Components))
	for _, component := range response.Components {
		components = append(components, agui.Component{
			Type: component.Type,
			Data: component.Data,
		})
	}
	response.Events = agui.EventsFor(response.State, response.Summary, components)
	return response
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

func looksLikeExistenceQuery(message string) bool {
	return strings.Contains(message, "already have") ||
		strings.Contains(message, "do i have") ||
		strings.Contains(message, "do we have") ||
		strings.Contains(message, "have this") ||
		strings.Contains(message, "own this") ||
		strings.Contains(message, "in my inventory")
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

func queryResultFromModel(modelResult modelQueryResult, category store.Category, items []store.Item) QueryResult {
	matches := make([]store.Item, 0, len(modelResult.MatchedItemIDs))
	seen := map[string]bool{}
	for _, id := range modelResult.MatchedItemIDs {
		for _, item := range items {
			if item.ID == id && !seen[id] {
				matches = append(matches, item)
				seen[id] = true
			}
		}
	}
	if modelResult.Summary == "" {
		modelResult.Summary = defaultQuerySummary(modelResult.Judgment, len(matches))
	}
	return QueryResult{
		Judgment:   modelResult.Judgment,
		Confidence: modelResult.Confidence,
		Summary:    modelResult.Summary,
		Category:   category,
		Matches:    matches,
	}
}

func defaultQuerySummary(judgment string, matchCount int) string {
	switch judgment {
	case "yes":
		return fmt.Sprintf("Found %d matching item(s).", matchCount)
	case "uncertain":
		return fmt.Sprintf("Found %d possible match(es).", matchCount)
	default:
		return "No matching item appears to be in inventory."
	}
}

func validJudgment(judgment string) bool {
	return judgment == "yes" || judgment == "no" || judgment == "uncertain"
}

func fallbackMatches(text string, items []store.Item) []store.Item {
	terms := significantTerms(text)
	if len(terms) == 0 {
		return nil
	}
	matches := make([]store.Item, 0)
	for _, item := range items {
		score := 0
		itemText := strings.ToLower(item.Title + " " + string(item.Attributes))
		for _, term := range terms {
			if strings.Contains(itemText, term) {
				score++
			}
		}
		if score >= minInt(2, len(terms)) {
			matches = append(matches, item)
		}
	}
	return matches
}

func significantTerms(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	terms := make([]string, 0, len(words))
	stop := map[string]bool{
		"add": true, "my": true, "the": true, "for": true, "with": true, "this": true,
		"do": true, "i": true, "have": true, "already": true, "show": true, "me": true,
		"inventory": true, "sealed": true, "open": true, "opened": true,
	}
	for _, word := range words {
		if len(word) < 3 || stop[word] {
			continue
		}
		terms = append(terms, word)
	}
	return terms
}

func normalizedTitle(title string) string {
	return strings.Join(significantTerms(title), " ")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
