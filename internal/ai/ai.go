// SPDX-FileCopyrightText: © 2026 Readeck custom build
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package ai contains a small provider-agnostic client used by the optional
// "AI cleanup" action. It only sends article text and image URLs; no cookies,
// archive bytes or user credentials leave the server.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config is a user's AI settings.
type Config struct {
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
	Timeout  time.Duration
}

// Image is one picture referenced by the article outline.
type Image struct {
	URL    string `json:"url"`
	Alt    string `json:"alt,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// Block is one block returned by the model. Only known types are rendered.
type Block struct {
	Type    string   `json:"type"`
	Text    string   `json:"text,omitempty"`
	Index   int      `json:"index,omitempty"`
	Caption string   `json:"caption,omitempty"`
	Items   []string `json:"items,omitempty"`
}

// Result is the JSON contract returned by every provider.
type Result struct {
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Labels  []string `json:"labels"`
	Blocks  []Block  `json:"blocks"`
}

var (
	// ErrNotConfigured is returned when the user has no provider/key yet.
	ErrNotConfigured = errors.New("AI is not configured")
	// ErrBadResponse is returned when the provider does not return usable JSON.
	ErrBadResponse = errors.New("the AI response could not be parsed")
)

// Client calls one provider.
type Client struct {
	cfg  Config
	http *http.Client
}

// New creates a client and fills provider defaults.
func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120 * time.Second
	}
	c := &Client{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
	c.normalize()
	return c
}

// Configured reports whether the minimum fields are present.
func (c *Client) Configured() bool {
	if c.cfg.Provider == "" {
		return false
	}
	// Ollama and a local vLLM usually have no key. A custom remote gateway
	// should set one; the request will simply fail with 401 if it is missing.
	if c.cfg.Provider == "ollama" || c.cfg.Provider == "custom" {
		return c.cfg.BaseURL != ""
	}
	return c.cfg.APIKey != ""
}

func (c *Client) normalize() {
	c.cfg.Provider = strings.ToLower(strings.TrimSpace(c.cfg.Provider))
	if c.cfg.Provider == "" {
		c.cfg.Provider = "openai"
	}
	c.cfg.BaseURL = strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
	c.cfg.Model = strings.TrimSpace(c.cfg.Model)

	switch c.cfg.Provider {
	case "openai":
		if c.cfg.BaseURL == "" {
			c.cfg.BaseURL = "https://api.openai.com/v1"
		}
		if c.cfg.Model == "" {
			c.cfg.Model = "gpt-4o-mini"
		}
	case "deepseek":
		if c.cfg.BaseURL == "" {
			c.cfg.BaseURL = "https://api.deepseek.com/v1"
		}
		if c.cfg.Model == "" {
			c.cfg.Model = "deepseek-chat"
		}
	case "moonshot":
		if c.cfg.BaseURL == "" {
			c.cfg.BaseURL = "https://api.moonshot.cn/v1"
		}
		if c.cfg.Model == "" {
			c.cfg.Model = "moonshot-v1-8k"
		}
	case "siliconflow":
		if c.cfg.BaseURL == "" {
			c.cfg.BaseURL = "https://api.siliconflow.cn/v1"
		}
		if c.cfg.Model == "" {
			c.cfg.Model = "Qwen/Qwen3.8-27B"
		}
	case "ollama":
		if c.cfg.BaseURL == "" {
			c.cfg.BaseURL = "http://localhost:11434"
		}
		if c.cfg.Model == "" {
			c.cfg.Model = "llama3.1"
		}
	case "anthropic":
		if c.cfg.BaseURL == "" {
			c.cfg.BaseURL = "https://api.anthropic.com"
		}
		if c.cfg.Model == "" {
			c.cfg.Model = "claude-3-5-haiku-latest"
		}
	case "custom":
		// Keep the user's URL and model exactly; this covers vLLM, LM Studio,
		// one-api and any other OpenAI-compatible gateway.
	}
}

func (c *Client) chatEndpoint() string {
	switch c.cfg.Provider {
	case "anthropic":
		return c.cfg.BaseURL + "/v1/messages"
	case "ollama":
		return c.cfg.BaseURL + "/api/chat"
	default:
		base := c.cfg.BaseURL
		if !strings.Contains(base, "/v1") && c.cfg.Provider != "custom" {
			base += "/v1"
		}
		return base + "/chat/completions"
	}
}

const systemPrompt = `You are a careful Chinese web-page editor. You receive a title, a URL,
an article outline and a numbered list of pictures. Remove navigation, author cards, player
chrome, ratings, reply buttons, ads, related links and copyright footers. Keep every real
paragraph, heading, list, code block and picture. Do not invent facts or pictures.

Return ONLY a JSON object, with no markdown fences:
{"title":"optional improved title","summary":"one sentence","labels":["tag"],
 "blocks":[{"type":"p","text":"..."},{"type":"h2","text":"..."},
           {"type":"quote","text":"..."},{"type":"list","items":["..."]},
           {"type":"img","index":0,"caption":"optional"}]}

Every [IMG n] in the outline must appear exactly once as an img block with that index.
Use the language of the article for title, summary, labels and text.`

// Clean asks the model to turn an outline into structured blocks.
func (c *Client) Clean(ctx context.Context, title, pageURL, outline string, images []Image) (*Result, error) {
	return c.CleanWithInstruction(ctx, title, pageURL, outline, images, "")
}

// CleanWithInstruction is [Clean] with an extra user instruction, such as
// "remove every link" or "keep only the recipe".
func (c *Client) CleanWithInstruction(ctx context.Context, title, pageURL, outline string, images []Image, instruction string) (*Result, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}

	user := fmt.Sprintf("标题: %s\nURL: %s\n\n正文大纲:\n%s\n\n图片列表:\n%s",
		title, pageURL, outline, imageList(images))
	if strings.TrimSpace(instruction) != "" {
		user += "\n\n额外要求（优先级最高）:\n" + strings.TrimSpace(instruction)
	}

	var body []byte
	var err error
	switch c.cfg.Provider {
	case "anthropic":
		body, err = c.callAnthropic(ctx, user)
	case "ollama":
		body, err = c.callOllama(ctx, user)
	default:
		body, err = c.callOpenAI(ctx, user)
	}
	if err != nil {
		return nil, err
	}

	res, err := parseResult(body)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Ask sends a free-form question. It is used by the list assistant and by the
// connection test, where structured article blocks are not needed.
func (c *Client) Ask(ctx context.Context, system, user string) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}

	var raw []byte
	var err error
	switch c.cfg.Provider {
	case "anthropic":
		payload := map[string]any{
			"model":      c.cfg.Model,
			"max_tokens": 4096,
			"system":     system,
			"messages": []map[string]string{
				{"role": "user", "content": user},
			},
		}
		raw, err = c.postJSON(ctx, c.chatEndpoint(), map[string]string{
			"x-api-key":         c.cfg.APIKey,
			"anthropic-version": "2023-06-01",
		}, payload)
	case "ollama":
		payload := map[string]any{
			"model":  c.cfg.Model,
			"stream": false,
			"messages": []map[string]string{
				{"role": "system", "content": system},
				{"role": "user", "content": user},
			},
		}
		raw, err = c.postJSON(ctx, c.chatEndpoint(), nil, payload)
	default:
		payload := map[string]any{
			"model": c.cfg.Model,
			"messages": []map[string]string{
				{"role": "system", "content": system},
				{"role": "user", "content": user},
			},
			"temperature": 0.2,
		}
		raw, err = c.postJSON(ctx, c.chatEndpoint(), map[string]string{
			"Authorization": "Bearer " + c.cfg.APIKey,
		}, payload)
	}
	if err != nil {
		return "", err
	}
	return contentFromResponse(raw)
}

// Test verifies the key, base URL and model with a tiny request.
func (c *Client) Test(ctx context.Context) error {
	_, err := c.Ask(ctx, "You are a connection tester. Reply with the single word OK.", "OK")
	return err
}

func contentFromResponse(raw []byte) (string, error) {
	text := strings.TrimSpace(string(raw))
	var wrapper struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(text), &wrapper); err != nil {
		return "", fmt.Errorf("%w: %v", ErrBadResponse, err)
	}
	switch {
	case len(wrapper.Choices) > 0:
		return wrapper.Choices[0].Message.Content, nil
	case len(wrapper.Content) > 0:
		return wrapper.Content[0].Text, nil
	default:
		return wrapper.Message.Content, nil
	}
}

func imageList(images []Image) string {
	var b strings.Builder
	for i, img := range images {
		fmt.Fprintf(&b, "[IMG %d] %s", i, img.URL)
		if img.Alt != "" {
			fmt.Fprintf(&b, " (%s)", img.Alt)
		}
		if img.Width > 0 && img.Height > 0 {
			fmt.Fprintf(&b, " %dx%d", img.Width, img.Height)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (c *Client) callOpenAI(ctx context.Context, user string) ([]byte, error) {
	payload := map[string]any{
		"model": c.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": user},
		},
		"temperature":     0.2,
		"response_format": map[string]string{"type": "json_object"},
	}
	return c.postJSON(ctx, c.chatEndpoint(), map[string]string{
		"Authorization": "Bearer " + c.cfg.APIKey,
	}, payload)
}

func (c *Client) callAnthropic(ctx context.Context, user string) ([]byte, error) {
	payload := map[string]any{
		"model":      c.cfg.Model,
		"max_tokens": 8192,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": user},
		},
	}
	return c.postJSON(ctx, c.chatEndpoint(), map[string]string{
		"x-api-key":         c.cfg.APIKey,
		"anthropic-version": "2023-06-01",
	}, payload)
}

func (c *Client) callOllama(ctx context.Context, user string) ([]byte, error) {
	payload := map[string]any{
		"model":  c.cfg.Model,
		"stream": false,
		"format": "json",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": user},
		},
		"options": map[string]any{"temperature": 0.2},
	}
	return c.postJSON(ctx, c.chatEndpoint(), nil, payload)
}

func (c *Client) postJSON(ctx context.Context, url string, headers map[string]string, payload map[string]any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rsp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AI request: %w", err)
	}
	defer rsp.Body.Close() //nolint:errcheck
	data, err := io.ReadAll(io.LimitReader(rsp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if rsp.StatusCode/100 != 2 {
		msg := strings.TrimSpace(string(data))
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return nil, fmt.Errorf("AI HTTP %d: %s", rsp.StatusCode, msg)
	}
	return data, nil
}

func parseResult(raw []byte) (*Result, error) {
	// Some providers still wrap JSON in a markdown fence.
	text := strings.TrimSpace(string(raw))
	if i := strings.Index(text, "{"); i >= 0 {
		text = text[i:]
	}
	if i := strings.LastIndex(text, "}"); i >= 0 {
		text = text[:i+1]
	}

	var wrapper struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(text), &wrapper); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadResponse, err)
	}

	content := text
	switch {
	case len(wrapper.Choices) > 0 && wrapper.Choices[0].Message.Content != "":
		content = wrapper.Choices[0].Message.Content
	case len(wrapper.Content) > 0 && wrapper.Content[0].Text != "":
		content = wrapper.Content[0].Text
	case wrapper.Message.Content != "":
		content = wrapper.Message.Content
	}

	content = strings.TrimSpace(content)
	if i := strings.Index(content, "{"); i >= 0 {
		content = content[i:]
	}
	if i := strings.LastIndex(content, "}"); i >= 0 {
		content = content[:i+1]
	}

	var res Result
	if err := json.Unmarshal([]byte(content), &res); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadResponse, err)
	}
	if len(res.Blocks) == 0 {
		return nil, ErrBadResponse
	}
	if len(res.Blocks) > 300 {
		res.Blocks = res.Blocks[:300]
	}
	return &res, nil
}
