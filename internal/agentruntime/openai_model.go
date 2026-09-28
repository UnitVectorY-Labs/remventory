package agentruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net/http"
	"strings"
	"time"

	"github.com/UnitVectorY-Labs/remventory/internal/config"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type OpenAIModel struct {
	cfg    config.Config
	client *http.Client
}

func NewOpenAIModel(cfg config.Config) *OpenAIModel {
	return &OpenAIModel{cfg: cfg, client: &http.Client{Timeout: 120 * time.Second}}
}
func (m *OpenAIModel) Name() string { return m.cfg.MainModel }

func (m *OpenAIModel) GenerateContent(ctx context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		messages := make([]map[string]any, 0, len(req.Contents)+1)
		if req.Config != nil && req.Config.SystemInstruction != nil {
			messages = append(messages, map[string]any{"role": "system", "content": contentText(req.Config.SystemInstruction)})
		}
		for _, c := range req.Contents {
			if c == nil {
				continue
			}
			role := c.Role
			if role == "model" {
				role = "assistant"
			}
			if role == "" {
				role = "user"
			}
			content := contentText(c)
			msg := map[string]any{"role": role}
			if content != "" {
				msg["content"] = content
			}
			calls := []map[string]any{}
			responses := []map[string]any{}
			for _, p := range c.Parts {
				if p == nil {
					continue
				}
				if p.FunctionCall != nil {
					args, _ := json.Marshal(p.FunctionCall.Args)
					calls = append(calls, map[string]any{"id": p.FunctionCall.ID, "type": "function", "function": map[string]any{"name": p.FunctionCall.Name, "arguments": string(args)}})
				}
				if p.FunctionResponse != nil {
					out, _ := json.Marshal(p.FunctionResponse.Response)
					responses = append(responses, map[string]any{"role": "tool", "tool_call_id": p.FunctionResponse.ID, "content": string(out)})
				}
			}
			if len(calls) > 0 {
				msg["tool_calls"] = calls
			}
			if len(msg) > 1 {
				messages = append(messages, msg)
			}
			messages = append(messages, responses...)
		}
		modelName := strings.TrimSpace(req.Model)
		if modelName == "" {
			modelName = m.cfg.MainModel
		}
		payload := map[string]any{"model": modelName, "messages": messages, "stream": false, "parallel_tool_calls": false, "tool_choice": "auto"}
		if req.Config != nil {
			if req.Config.Temperature != nil {
				payload["temperature"] = *req.Config.Temperature
			}
			if req.Config.MaxOutputTokens > 0 {
				payload["max_tokens"] = req.Config.MaxOutputTokens
			}
			if len(req.Config.Tools) > 0 {
				tools := []map[string]any{}
				for _, t := range req.Config.Tools {
					if t == nil {
						continue
					}
					for _, f := range t.FunctionDeclarations {
						if f == nil {
							continue
						}
						params := f.ParametersJsonSchema
						if params == nil {
							params = f.Parameters
						}
						tools = append(tools, map[string]any{"type": "function", "function": map[string]any{"name": f.Name, "description": f.Description, "parameters": params}})
					}
				}
				if len(tools) > 0 {
					payload["tools"] = tools
				}
			}
		}
		data, err := json.Marshal(payload)
		if err != nil {
			yield(nil, err)
			return
		}
		url := strings.TrimRight(m.cfg.OpenAIBaseURL, "/") + "/chat/completions"
		r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			yield(nil, err)
			return
		}
		r.Header.Set("Content-Type", "application/json")
		if m.cfg.OpenAIAPIKey != "" {
			r.Header.Set("Authorization", "Bearer "+m.cfg.OpenAIAPIKey)
		}
		resp, err := m.client.Do(r)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			yield(nil, errors.New("model endpoint returned "+resp.Status))
			return
		}
		var parsed struct {
			Choices []struct {
				FinishReason string `json:"finish_reason"`
				Message      struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
		if err != nil {
			yield(nil, err)
			return
		}
		if len(body) > 4<<20 {
			yield(nil, errors.New("model response exceeded its size limit"))
			return
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			yield(nil, err)
			return
		}
		if len(parsed.Choices) == 0 {
			yield(nil, errors.New("model returned no choices"))
			return
		}
		if parsed.Choices[0].FinishReason == "length" {
			yield(nil, errors.New("model answer exceeded its output limit"))
			return
		}
		msg := parsed.Choices[0].Message
		content := stripThinking(msg.Content)
		parts := []*genai.Part{}
		if strings.TrimSpace(content) != "" {
			parts = append(parts, genai.NewPartFromText(content))
		}
		for _, call := range msg.ToolCalls {
			if strings.TrimSpace(call.ID) == "" || strings.TrimSpace(call.Function.Name) == "" {
				yield(nil, errors.New("model returned an invalid tool call"))
				return
			}
			var args map[string]any
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				yield(nil, err)
				return
			}
			parts = append(parts, &genai.Part{FunctionCall: &genai.FunctionCall{ID: strings.TrimSpace(call.ID), Name: strings.TrimSpace(call.Function.Name), Args: args}})
		}
		if len(parts) == 0 {
			yield(nil, errors.New("model returned an empty response"))
			return
		}
		if !yield(&model.LLMResponse{Content: genai.NewContentFromParts(parts, genai.RoleModel), TurnComplete: true}, nil) {
			return
		}
	}
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

func contentText(c *genai.Content) string {
	var b strings.Builder
	for _, p := range c.Parts {
		if p != nil && p.Text != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(p.Text)
		}
	}
	return b.String()
}
