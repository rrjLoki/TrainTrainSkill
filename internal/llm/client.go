// Package llm 实现一个最小的 OpenAI 兼容客户端。
//
// 目标是能同时对接 vLLM、Ollama 与云端 API：只依赖 /chat/completions
// 这一个约定，换后端只需改 LLM_BASE_URL。
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message 是一条对话消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client 是 OpenAI 兼容的对话客户端。
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// NewClient 创建客户端。baseURL 形如 http://127.0.0.1:8000/v1。
func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		// 不设 Timeout：流式响应可能持续数分钟，超时交给调用方的 context 控制。
		http: &http.Client{},
	}
}

// Model 返回当前使用的模型名。
func (c *Client) Model() string { return c.model }

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
		Delta   Message `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 发起一次非流式对话，返回完整回复文本。
func (c *Client) Chat(ctx context.Context, msgs []Message) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	body, err := json.Marshal(chatRequest{Model: c.model, Messages: msgs})
	if err != nil {
		return "", fmt.Errorf("llm: encode request: %w", err)
	}

	req, err := c.newRequest(ctx, body)
	if err != nil {
		return "", err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", statusError(resp)
	}

	var out chatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&out); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("llm: upstream error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("llm: empty choices")
	}
	return out.Choices[0].Message.Content, nil
}

// ChatStream 发起流式对话，每收到一段增量文本就调用一次 onDelta。
// 调用方负责用 context 控制总时长。
func (c *Client) ChatStream(ctx context.Context, msgs []Message, onDelta func(delta string) error) error {
	body, err := json.Marshal(chatRequest{Model: c.model, Messages: msgs, Stream: true})
	if err != nil {
		return fmt.Errorf("llm: encode request: %w", err)
	}

	req, err := c.newRequest(ctx, body)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("llm: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return statusError(resp)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		payload, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		payload = strings.TrimSpace(payload)
		if payload == "[DONE]" {
			return nil
		}

		var chunk chatResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return fmt.Errorf("llm: decode stream chunk: %w", err)
		}
		if chunk.Error != nil {
			return fmt.Errorf("llm: upstream error: %s", chunk.Error.Message)
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content == "" {
				continue
			}
			if err := onDelta(ch.Delta.Content); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("llm: read stream: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("llm: stream interrupted: %w", err)
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return req, nil
}

func statusError(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	msg := strings.TrimSpace(string(b))
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("llm: unexpected status %d: %s", resp.StatusCode, msg)
}
