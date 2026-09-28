package remy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/UnitVectorY-Labs/remventory/internal/agentruntime"
	"github.com/UnitVectorY-Labs/remventory/internal/store"
	"github.com/google/uuid"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

type remyFunctionTool struct {
	declaration *genai.FunctionDeclaration
	run         func(agent.ToolContext, any) (map[string]any, error)
}

func (t *remyFunctionTool) Name() string                            { return t.declaration.Name }
func (t *remyFunctionTool) Description() string                     { return t.declaration.Description }
func (t *remyFunctionTool) IsLongRunning() bool                     { return false }
func (t *remyFunctionTool) Declaration() *genai.FunctionDeclaration { return t.declaration }
func (t *remyFunctionTool) ProcessRequest(_ agent.ToolContext, req *model.LLMRequest) error {
	if req.Tools == nil {
		req.Tools = make(map[string]any)
	}
	if _, exists := req.Tools[t.Name()]; exists {
		return fmt.Errorf("duplicate tool: %q", t.Name())
	}
	req.Tools[t.Name()] = t
	if req.Config == nil {
		req.Config = &genai.GenerateContentConfig{}
	}
	for _, configured := range req.Config.Tools {
		if configured != nil && configured.FunctionDeclarations != nil {
			configured.FunctionDeclarations = append(configured.FunctionDeclarations, t.declaration)
			return nil
		}
	}
	req.Config.Tools = append(req.Config.Tools, &genai.Tool{FunctionDeclarations: []*genai.FunctionDeclaration{t.declaration}})
	return nil
}
func (t *remyFunctionTool) Run(ctx agent.ToolContext, args any) (map[string]any, error) {
	return t.run(ctx, args)
}

func (s *Service) handleAgent(ctx context.Context, req Request, message string) (Response, error) {
	if !s.modelConfigured(s.cfg.MainModel) {
		return Response{}, errors.New("Remy is not configured with a language model")
	}
	conversationID := strings.TrimSpace(req.SessionID)
	if _, err := uuid.Parse(conversationID); err != nil {
		conversationID = uuid.NewString()
	}
	history, err := s.store.Conversation(ctx, conversationID)
	if err != nil {
		return Response{}, err
	}
	pending, err := s.store.ListPendingProposals(ctx, 30)
	if err != nil {
		return Response{}, err
	}
	focusIDs := append([]string{}, req.Focus...)
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			focusIDs = append(focusIDs, history[i].References...)
			break
		}
	}
	focus, err := s.hydrateFocus(ctx, uniqueStrings(focusIDs))
	if err != nil {
		return Response{}, err
	}
	var prompt strings.Builder
	prompt.WriteString(remyAgentInstructions + "\nCurrent pending proposals: " + mustJSON(pending) + "\nCurrent browser focus (hydrated from canonical IDs): " + mustJSON(focus) + "\nBounded recent conversation:\n")
	for _, turn := range history {
		fmt.Fprintf(&prompt, "%s: %s\n", turn.Role, turn.Content)
	}
	fmt.Fprintf(&prompt, "Current request: %s", message)
	evidenceByID := map[string]evidence{}
	components := []Component{}
	seq := 0
	var toolMu sync.Mutex
	toolCalls := 0
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	definitions := s.agentToolDefinitions()
	tools := make([]tool.Tool, 0, len(definitions))
	for _, definition := range definitions {
		function := definition["function"].(map[string]any)
		name := function["name"].(string)
		description := function["description"].(string)
		parameters := function["parameters"].(map[string]any)
		decl := &genai.FunctionDeclaration{Name: name, Description: description, ParametersJsonSchema: parameters}
		toolImpl := &remyFunctionTool{declaration: decl}
		toolImpl.run = func(_ agent.ToolContext, args any) (map[string]any, error) {
			toolMu.Lock()
			toolCalls++
			tooMany := toolCalls > 24
			toolMu.Unlock()
			if tooMany {
				cancel()
				return nil, errors.New("Remy reached its tool limit")
			}
			values, ok := args.(map[string]any)
			if !ok {
				return nil, errors.New("tool arguments must be an object")
			}
			toolMu.Lock()
			result, component, err := s.executeAgentTool(runCtx, name, values, &evidenceByID, &seq)
			if component != nil {
				components = append(components, *component)
			}
			toolMu.Unlock()
			if err != nil {
				return map[string]any{"error": err.Error()}, nil
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				return nil, err
			}
			var normalized map[string]any
			if err := json.Unmarshal(encoded, &normalized); err != nil {
				return nil, err
			}
			return normalized, nil
		}
		tools = append(tools, toolImpl)
	}
	temp := float32(.3)
	a, err := llmagent.New(llmagent.Config{Name: "remy", Description: "Natural language inventory assistant that chooses typed presentations", Model: agentruntime.NewOpenAIModel(s.cfg), Instruction: "Use tools for canonical facts. Answer naturally and choose a typed presentation when useful. Never invent data or commit writes.", Tools: tools, GenerateContentConfig: &genai.GenerateContentConfig{Temperature: &temp, MaxOutputTokens: 2048}})
	if err != nil {
		return Response{}, err
	}
	r, err := runner.New(runner.Config{AppName: "remventory", Agent: a, SessionService: session.InMemoryService(), AutoCreateSession: true})
	if err != nil {
		return Response{}, err
	}
	var answer strings.Builder
	for event, runErr := range r.Run(runCtx, "remy", uuid.NewString(), genai.NewContentFromText(prompt.String(), genai.RoleUser), agent.RunConfig{}) {
		if runErr != nil {
			return Response{}, fmt.Errorf("Remy could not complete the request: %w", runErr)
		}
		if event != nil && event.IsFinalResponse() && event.Content != nil {
			for _, part := range event.Content.Parts {
				if part != nil && part.Text != "" {
					answer.WriteString(part.Text)
				}
			}
		}
	}
	final := strings.TrimSpace(stripThinking(answer.String()))
	if final == "" {
		return Response{}, errors.New("model returned no final answer")
	}
	if err := s.store.AppendConversationTurns(ctx, conversationID, store.ConversationTurn{Role: "user", Content: boundedText(message, 3000)}, store.ConversationTurn{Role: "assistant", Content: boundedText(final, 3000), References: componentReferences(components)}); err != nil {
		return Response{}, err
	}
	return withEvents(Response{State: "completed", Summary: final, RequestSummary: truncate(message, 80), Components: components, SessionID: conversationID}), nil
}

func boundedText(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}

func stripThinking(text string) string {
	for {
		start := strings.Index(text, "<think>")
		if start < 0 {
			break
		}
		end := strings.Index(text[start+len("<think>"):], "</think>")
		if end < 0 {
			text = text[:start]
			break
		}
		end += start + len("<think>")
		text = text[:start] + text[end+len("</think>"):]
	}
	return strings.TrimSpace(strings.ReplaceAll(text, "</think>", ""))
}

func componentReferences(components []Component) []string {
	set := map[string]bool{}
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				if key == "id" || key == "item_id" || key == "category_id" {
					if id, ok := child.(string); ok && uuid.Validate(id) == nil {
						set[id] = true
					}
				}
				if key == "item_ids" {
					if list, ok := child.([]string); ok {
						for _, id := range list {
							if uuid.Validate(id) == nil {
								set[id] = true
							}
						}
					}
					if list, ok := child.([]any); ok {
						for _, value := range list {
							if id, ok := value.(string); ok && uuid.Validate(id) == nil {
								set[id] = true
							}
						}
					}
				}
				visit(child)
			}
		case []any:
			for _, child := range v {
				visit(child)
			}
		case []store.Item:
			for _, item := range v {
				set[item.ID] = true
				set[item.CategoryID] = true
			}
		case store.Item:
			set[v.ID] = true
			set[v.CategoryID] = true
		case store.Category:
			set[v.ID] = true
		case []store.Category:
			for _, category := range v {
				set[category.ID] = true
			}
		case store.Proposal:
			set[v.ID] = true
		}
	}
	for _, component := range components {
		visit(component.Data)
	}
	refs := make([]string, 0, len(set))
	for id := range set {
		refs = append(refs, id)
	}
	if len(refs) > 30 {
		refs = refs[:30]
	}
	return refs
}

func uniqueStrings(input []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(input))
	for _, value := range input {
		if _, err := uuid.Parse(value); err != nil || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
