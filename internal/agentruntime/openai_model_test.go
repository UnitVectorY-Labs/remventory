package agentruntime

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/UnitVectorY-Labs/remventory/internal/config"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func response(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestOpenAIModelConvertsToolCallsAndStripsThinking(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		return response(`{"choices":[{"finish_reason":"tool_calls","message":{"content":"<think>private reasoning</think>Checking now","reasoning_content":"separate reasoning","tool_calls":[{"id":"call-1","type":"function","function":{"name":"search_inventory","arguments":"{\"query\":\"lamp\"}"}}]}}]}`), nil
	})}
	llm := NewOpenAIModel(config.Config{OpenAIBaseURL: "http://model.invalid/v1", MainModel: "test-model"})
	llm.client = client
	var got *model.LLMResponse
	for item, err := range llm.GenerateContent(context.Background(), &model.LLMRequest{Contents: []*genai.Content{genai.NewContentFromText("find lamps", genai.RoleUser)}}, false) {
		if err != nil {
			t.Fatal(err)
		}
		got = item
	}
	if got == nil || got.Content == nil {
		t.Fatal("missing model response")
	}
	if len(got.Content.Parts) != 2 {
		t.Fatalf("parts = %d, want text plus tool call", len(got.Content.Parts))
	}
	if got.Content.Parts[0].Text != "Checking now" {
		t.Fatalf("thinking text leaked or final text lost: %q", got.Content.Parts[0].Text)
	}
	call := got.Content.Parts[1].FunctionCall
	if call == nil || call.ID != "call-1" || call.Name != "search_inventory" || call.Args["query"] != "lamp" {
		t.Fatalf("invalid converted call: %#v", call)
	}
}

func TestOpenAIModelRejectsTruncatedAndEmptyResponses(t *testing.T) {
	for _, tc := range []struct{ name, response string }{{"truncated", `{"choices":[{"finish_reason":"length","message":{"content":"partial"}}]}`}, {"empty", `{"choices":[{"finish_reason":"stop","message":{"content":"<think>hidden</think>"}}]}`}} {
		t.Run(tc.name, func(t *testing.T) {
			llm := NewOpenAIModel(config.Config{OpenAIBaseURL: "http://model.invalid", MainModel: "test"})
			llm.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return response(tc.response), nil })}
			count := 0
			for _, err := range llm.GenerateContent(context.Background(), &model.LLMRequest{Model: "test", Contents: []*genai.Content{genai.NewContentFromText("x", genai.RoleUser)}}, false) {
				if err != nil {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("errors=%d, want one", count)
			}
		})
	}
}

func TestOpenAIModelRejectsOversizedResponseAndInvalidToolCall(t *testing.T) {
	for _, body := range []string{`{"choices":[{"finish_reason":"stop","message":{"content":"` + strings.Repeat("x", 4<<20) + `"}}]}`, `{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"id":"","function":{"name":"search_inventory","arguments":"{}"}}]}}]}`} {
		llm := NewOpenAIModel(config.Config{OpenAIBaseURL: "http://model.invalid", MainModel: "test"})
		llm.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return response(body), nil })}
		gotErr := false
		for _, err := range llm.GenerateContent(context.Background(), &model.LLMRequest{Model: "test", Contents: []*genai.Content{genai.NewContentFromText("x", genai.RoleUser)}}, false) {
			if err != nil {
				gotErr = true
			}
		}
		if !gotErr {
			t.Fatal("expected malformed or oversized response to fail")
		}
	}
}
