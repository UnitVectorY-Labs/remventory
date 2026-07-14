package remy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/UnitVectorY-Labs/remventory/internal/config"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
)

func TestPlanRequestUsesModelStructuredOutput(t *testing.T) {
	var requestBody map[string]any
	service := New(config.Config{OpenAIBaseURL: "http://model.test/v1", MainModel: "main-model"}, nil)
	service.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatal(err)
		}
		var body bytes.Buffer
		_ = json.NewEncoder(&body).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{"content": `{"action":"item_change"}`},
			}},
		})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(&body)}, nil
	})}
	action, err := service.planRequest(context.Background(), "Please put another drill in the workshop inventory", []store.Category{{Name: "Workshop tools"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if action != "item_change" {
		t.Fatalf("action = %q, want item_change", action)
	}
	format, ok := requestBody["response_format"].(map[string]any)
	if !ok || format["type"] != "json_object" {
		t.Fatalf("response_format = %#v, want json_object", requestBody["response_format"])
	}
	if requestBody["model"] != "main-model" {
		t.Fatalf("model = %q, want main-model", requestBody["model"])
	}
}

func TestPlanRequestFallbacks(t *testing.T) {
	service := New(config.Config{}, nil)
	categories := []store.Category{{Name: "Board games"}}
	tests := []struct {
		message string
		action  string
	}{
		{"I want to track my board games", "category_change"},
		{"Do I already have Catan?", "query_inventory"},
		{"Show me my board games", "list_items"},
		{"What attributes does board games have?", "category_definition"},
		{"Add Catan", "item_change"},
		{"Hello there", "help"},
	}
	for _, test := range tests {
		t.Run(test.action, func(t *testing.T) {
			action, err := service.planRequest(context.Background(), test.message, categories, nil)
			if err != nil {
				t.Fatal(err)
			}
			if action != test.action {
				t.Fatalf("action = %q, want %q", action, test.action)
			}
		})
	}
}

func TestItemAttributesOnlyUseCategoryDefinition(t *testing.T) {
	attributes, err := itemAttributesForCategory(
		json.RawMessage(`{"condition":"used","made_up":"value"}`),
		store.Category{Attributes: []store.Attribute{{Key: "condition"}, {Key: "platform"}}},
		json.RawMessage(`{"platform":"Switch","legacy":"remove me"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]string
	if err := json.Unmarshal(attributes, &values); err != nil {
		t.Fatal(err)
	}
	if values["condition"] != "used" || values["platform"] != "Switch" || len(values) != 2 {
		t.Fatalf("attributes = %#v", values)
	}
}

func TestExactItemMatchDoesNotGuess(t *testing.T) {
	items := []store.Item{{ID: "one", Title: "Catan"}, {ID: "two", Title: "Catan: Seafarers"}}
	if match := exactItemMatch("Catan", items); match == nil || match.ID != "one" {
		t.Fatalf("match = %#v", match)
	}
	if match := exactItemMatch("Catan expansion", items); match != nil {
		t.Fatalf("match = %#v, want nil", match)
	}
}

func TestDistinctiveTitleMatchAcceptsOneClearVariant(t *testing.T) {
	items := []store.Item{{ID: "going-merry", Title: "The Going Merry Pirate Ship"}, {ID: "thousand-sunny", Title: "Thousand Sunny"}}
	match := distinctiveTitleMatch("Going Merry LEGO set", items)
	if match == nil || match.ID != "going-merry" {
		t.Fatalf("match = %#v, want Going Merry", match)
	}
}

func TestDistinctiveTitleMatchRejectsAmbiguousVariant(t *testing.T) {
	items := []store.Item{{ID: "one", Title: "Going Merry Pirate Ship"}, {ID: "two", Title: "Going Merry Display Model"}}
	if match := distinctiveTitleMatch("Going Merry LEGO set", items); match != nil {
		t.Fatalf("match = %#v, want nil", match)
	}
}

func TestSelectCategoryUsesModelWhenNameIsNotInRequest(t *testing.T) {
	service := New(config.Config{OpenAIBaseURL: "http://model.test/v1", MainModel: "main-model"}, nil)
	service.client = jsonResponseClient(t, `{"category_id":"games-id"}`)
	categories := []store.Category{
		{ID: "tools-id", Name: "Workshop tools"},
		{ID: "games-id", Name: "My Video Games"},
	}

	category, err := service.selectCategory(context.Background(), "Do I own Super Mario Bros. Wonder for Nintendo Switch?", categories)
	if err != nil {
		t.Fatal(err)
	}
	if category == nil || category.ID != "games-id" {
		t.Fatalf("category = %#v, want games-id", category)
	}
}

func TestPendingProposalFromVisibleContext(t *testing.T) {
	visible := &VisibleContext{Components: []Component{{
		Type: "item_proposal",
		Data: map[string]any{"id": "proposal-id", "status": "pending", "type": "item_change"},
	}}}
	proposal, componentType, err := pendingProposalFromContext(visible)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.ID != "proposal-id" || componentType != "item_proposal" {
		t.Fatalf("proposal = %#v, componentType = %q", proposal, componentType)
	}
}

func TestTinyModelProducesRequestSummary(t *testing.T) {
	var usedModel string
	service := New(config.Config{OpenAIBaseURL: "http://model.test/v1", TinyModel: "tiny-model"}, nil)
	service.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var request map[string]any
		_ = json.NewDecoder(r.Body).Decode(&request)
		usedModel, _ = request["model"].(string)
		return jsonCompletionResponse(`{"summary":"Revise page count"}`), nil
	})}

	summary := service.summarizeRequest(context.Background(), "Please change the page count to 96")
	if summary != "Revise page count" || usedModel != "tiny-model" {
		t.Fatalf("summary = %q, model = %q", summary, usedModel)
	}
}

func TestThinkingModelHandlesInventoryMatch(t *testing.T) {
	var usedModel string
	service := New(config.Config{OpenAIBaseURL: "http://model.test/v1", ThinkingModel: "thinking-model"}, nil)
	service.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var request map[string]any
		_ = json.NewDecoder(r.Body).Decode(&request)
		usedModel, _ = request["model"].(string)
		return jsonCompletionResponse(`{"judgment":"yes","confidence":"high","summary":"Found it","matched_item_ids":["item-id"]}`), nil
	})}

	result, err := service.matchExistingItem(context.Background(), "Do I have it?", store.Category{ID: "category-id"}, []store.Item{{ID: "item-id", Title: "Item"}}, "Item")
	if err != nil {
		t.Fatal(err)
	}
	if result.Judgment != "yes" || usedModel != "thinking-model" {
		t.Fatalf("judgment = %q, model = %q", result.Judgment, usedModel)
	}
}

func jsonCompletionResponse(content string) *http.Response {
	var body bytes.Buffer
	_ = json.NewEncoder(&body).Encode(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
	})
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(&body)}
}

func jsonResponseClient(t *testing.T, content string) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body bytes.Buffer
		_ = json.NewEncoder(&body).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{"content": content},
			}},
		})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(&body)}, nil
	})}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
