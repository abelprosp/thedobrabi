package aiagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// callOpenAI calls the configured OpenAI chat/completions endpoint with a
// system+user prompt and asks for a JSON object response. It returns the raw
// response body on success. It is used by the Ask flow.
func (a *Agent) callOpenAI(ctx context.Context, system, user string) ([]byte, error) {
	if a.cfg.OpenAIKey == "" {
		return nil, fmt.Errorf("openai key not configured")
	}
	body, _ := json.Marshal(map[string]any{
		"model": a.cfg.OpenAIModel,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature":     0.1,
		"response_format": map[string]string{"type": "json_object"},
	})
	return a.doOpenAI(ctx, body)
}

// callOpenAIJSON calls OpenAI with a tool/function definition so the model must
// return a JSON object matching the supplied schema. It returns the parsed
// JSON arguments (or content). Use this when the response shape must be strictly
// validated, e.g. dashboard generation.
func (a *Agent) callOpenAIJSON(ctx context.Context, system, user string, schema map[string]any) ([]byte, error) {
	if a.cfg.OpenAIKey == "" {
		return nil, fmt.Errorf("openai key not configured")
	}
	body, _ := json.Marshal(map[string]any{
		"model": a.cfg.OpenAIModel,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0.1,
		"tools": []map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":        "render_json",
					"description": "Render the response as the requested JSON object.",
					"parameters":  schema,
				},
			},
		},
		"tool_choice": map[string]any{"type": "function", "function": map[string]any{"name": "render_json"}},
	})
	raw, err := a.doOpenAI(ctx, body)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("invalid llm response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty llm response")
	}
	msg := resp.Choices[0].Message
	if len(msg.ToolCalls) > 0 {
		return []byte(msg.ToolCalls[0].Function.Arguments), nil
	}
	return []byte(msg.Content), nil
}

func (a *Agent) doOpenAI(ctx context.Context, body []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(a.cfg.OpenAIBaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+a.cfg.OpenAIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("llm status %d: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}
