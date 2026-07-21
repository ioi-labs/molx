package cleaner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Config holds the OpenAI-compatible endpoint credentials.
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

// Client cleans Markdown using an LLM chat completion endpoint.
type Client struct {
	cfg    Config
	httpDo func(*http.Request) (*http.Response, error)
}

// NewClient creates a cleaner bound to the supplied config.
// ponytail: stdlib HTTP only; no OpenAI SDK dependency.
func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg, httpDo: http.DefaultClient.Do}
}

// chatRequest mirrors the OpenAI chat completions request shape.
type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse mirrors the smallest usable subset of the OpenAI response.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// CleanMarkdown asks the configured LLM to rewrite raw web markdown into clean,
// structured content: keep headings, paragraphs, and lists; remove ads,
// navigation, cookie banners, promotional noise, and redundant stats. Output
// plain Markdown only.
func (c *Client) CleanMarkdown(ctx context.Context, markdown string) (string, error) {
	if c.cfg.BaseURL == "" || c.cfg.APIKey == "" || c.cfg.Model == "" {
		return "", errors.New("LLM not provided")
	}

	payload := chatRequest{
		Model: c.cfg.Model,
		Messages: []message{
			{Role: "system", Content: cleanSystemPrompt},
			{Role: "user", Content: markdown},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := c.cfg.BaseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.httpDo(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM returned %d: %s", resp.StatusCode, string(respBody))
	}

	var out chatResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", errors.New(out.Error.Message)
	}
	if len(out.Choices) == 0 || out.Choices[0].Message.Content == "" {
		return "", errors.New("LLM returned empty content")
	}
	return out.Choices[0].Message.Content, nil
}

const cleanSystemPrompt = `You are a web content cleaner. Rewrite the raw Markdown below into clean, readable Markdown that keeps only the meaningful page content.

Rules:
- Preserve headings, paragraphs, bullet/numbered lists, and tables when they carry real information.
- Remove ads, navigation, cookie banners, newsletter signup boxes, promotional popups, social-share widgets, comments, footers, sidebars, and redundant marketing statistics.
- Collapse noisy numeric callouts (e.g., "2 / 8+ / 84%") into one short sentence only if they are clearly key facts; otherwise drop them.
- Do not add information that is not in the source. Do not summarize into a single paragraph unless the original is already short.
- Return only Markdown. Do not wrap the output in code fences. Do not add explanatory text.`
