package remy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestPlanRequestRejectsContextActionWithoutVisibleContent(t *testing.T) {
	service := New(config.Config{OpenAIBaseURL: "http://model.test/v1", MainModel: "main-model"}, nil)
	service.client = jsonResponseClient(t, `{"action":"answer_context"}`)
	action, err := service.planRequest(context.Background(), "Write me a poem", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if action != "help" {
		t.Fatalf("action = %q, want help", action)
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

func TestDialogUsesMainModelWithStrictSchemaAndHardLimit(t *testing.T) {
	var requestBody map[string]any
	service := New(config.Config{OpenAIBaseURL: "http://model.test/v1", MainModel: "main-model"}, nil)
	service.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		_ = json.NewDecoder(r.Body).Decode(&requestBody)
		return jsonCompletionResponse(`{"icon":"searching","message":"` + strings.Repeat("a", 160) + `"}`), nil
	})}

	dialog, err := service.Dialog(context.Background(), DialogRequest{Phase: "working", Message: "Do I have Catan?"})
	if err != nil {
		t.Fatal(err)
	}
	if dialog.Icon != "searching" || utf8.RuneCountInString(dialog.Message) != maxDialogCharacters {
		t.Fatalf("dialog = %#v, rune count = %d", dialog, utf8.RuneCountInString(dialog.Message))
	}
	if requestBody["model"] != "main-model" {
		t.Fatalf("model = %q, want main-model", requestBody["model"])
	}
	if requestBody["temperature"] != 0.7 {
		t.Fatalf("temperature = %#v, want 0.7", requestBody["temperature"])
	}
	messages := requestBody["messages"].([]any)
	userMessage := messages[1].(map[string]any)["content"].(string)
	var userPayload map[string]any
	if err := json.Unmarshal([]byte(userMessage), &userPayload); err != nil {
		t.Fatal(err)
	}
	if userPayload["variation_hint"] == "" {
		t.Fatal("dialog request is missing variation_hint")
	}
	format := requestBody["response_format"].(map[string]any)
	if format["type"] != "json_schema" {
		t.Fatalf("response_format = %#v, want json_schema", format)
	}
	definition := format["json_schema"].(map[string]any)
	if definition["strict"] != true {
		t.Fatalf("json_schema = %#v, want strict schema", definition)
	}
	schema := definition["schema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	messageSchema := properties["message"].(map[string]any)
	if messageSchema["maxLength"] != float64(maxDialogCharacters) {
		t.Fatalf("maxLength = %#v, want %d", messageSchema["maxLength"], maxDialogCharacters)
	}
}

func TestDialogPromptsRequireFirstPersonExamplesAndVariation(t *testing.T) {
	for _, name := range []string{"dialog_working.txt", "dialog_completed.txt"} {
		t.Run(name, func(t *testing.T) {
			raw, err := promptFiles.ReadFile("prompts/" + name)
			if err != nil {
				t.Fatal(err)
			}
			prompt := string(raw)
			for _, required := range []string{"first person", "GOOD EXAMPLES", "BAD EXAMPLES", "not templates", "Vary the opening", "organized, patient, and helpful", "Do not use the word \"quietly\""} {
				if !strings.Contains(prompt, required) {
					t.Fatalf("prompt is missing %q", required)
				}
			}
		})
	}
}

func TestDialogFallbackGuidesUnrelatedRequest(t *testing.T) {
	service := New(config.Config{OpenAIBaseURL: "http://model.test/v1", MainModel: "main-model"}, nil)
	service.client = jsonResponseClient(t, `{"icon":"cataloging","message":"A poem proposal is ready."}`)
	dialog, err := service.Dialog(context.Background(), DialogRequest{Phase: "completed", Message: "Write me a poem", Context: &VisibleContext{State: "completed"}})
	if err != nil {
		t.Fatal(err)
	}
	if dialog.Icon != "ready" || !strings.Contains(dialog.Message, "inventory") {
		t.Fatalf("dialog = %#v, want ready inventory guidance", dialog)
	}
}

func TestCompletedDialogFallbackKeepsRemysFirstPersonVoice(t *testing.T) {
	dialog := completedDialogFallback(&VisibleContext{
		State:      "completed",
		Components: []Component{{Type: "category_list", Data: []any{}}},
	})
	if !strings.Contains(dialog.Message, "I’ve") {
		t.Fatalf("dialog = %#v, want first-person fallback", dialog)
	}
}

func TestDialogIconMatchesDisplayedContent(t *testing.T) {
	service := New(config.Config{OpenAIBaseURL: "http://model.test/v1", MainModel: "main-model"}, nil)
	service.client = jsonResponseClient(t, `{"icon":"cataloging","message":"Choose a category to list."}`)
	dialog, err := service.Dialog(context.Background(), DialogRequest{
		Phase:   "completed",
		Message: "Show categories",
		Context: &VisibleContext{State: "completed", Components: []Component{{Type: "category_list", Data: []any{}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if dialog.Icon != "celebrating" {
		t.Fatalf("icon = %q, want celebrating", dialog.Icon)
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
