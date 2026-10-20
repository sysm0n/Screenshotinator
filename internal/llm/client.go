// Package llm talks to an OpenAI-compatible vision endpoint (e.g. a DGX-hosted server).
//
// The DGX endpoint accepts at most 2 images per request, so every request carries exactly ONE
// image and no history: each screenshot is its own isolated chat session.
package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	systemPrompt = "You are a technical documentation assistant. You are shown a single screenshot of the " +
		"active application window. Write a concise developer-oriented summary (2-4 sentences) of what the window " +
		"shows: the application, the visible UI state, relevant code/commands/errors/values, and what action the " +
		"user most likely just performed. Be factual; do not speculate beyond what is visible."

	maxTokens        = 600
	maxResponseBytes = 4 << 20
	maxErrorBody     = 512
)

// ErrEmptyResponse is returned when the model answers without any content.
var ErrEmptyResponse = errors.New("llm returned no content")

// Endpoint is where and how to reach the model. An empty APIKey sends no Authorization header.
type Endpoint struct {
	URL    string
	Model  string
	APIKey string
}

// Request is a single-screenshot analysis.
type Request struct {
	JPEG         []byte
	WindowTitle  string
	Instructions string // optional user guidance appended to the prompt
}

// StatusError is returned for non-2xx responses.
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string { return fmt.Sprintf("llm http %d: %s", e.Code, e.Body) }

// Client is safe for concurrent use. The endpoint is resolved per request, so settings
// changes apply without rebuilding the client.
type Client struct {
	http     *http.Client
	endpoint func() Endpoint
}

func New(timeout time.Duration, endpoint func() Endpoint) *Client {
	return &Client{http: &http.Client{Timeout: timeout}, endpoint: endpoint}
}

// Analyze sends one standalone single-image chat completion and returns the summary text.
func (c *Client) Analyze(ctx context.Context, req Request) (string, error) {
	ep := c.endpoint()
	body, err := json.Marshal(chatRequest{
		Model:     ep.Model,
		MaxTokens: maxTokens,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: []contentPart{
				{Type: "text", Text: userPrompt(req.WindowTitle, req.Instructions)},
				{Type: "image_url", ImageURL: &imageURL{URL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(req.JPEG)}},
			}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if ep.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+ep.APIKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return "", &StatusError{Code: resp.StatusCode, Body: truncate(strings.TrimSpace(string(raw)), maxErrorBody)}
	}

	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", ErrEmptyResponse
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		return "", ErrEmptyResponse
	}
	return text, nil
}

func userPrompt(windowTitle, instructions string) string {
	p := fmt.Sprintf("Active window: %q. Summarize this screenshot.", windowTitle)
	if instructions = strings.TrimSpace(instructions); instructions != "" {
		p += "\n\nThe user gave these additional instructions for this screenshot. Follow them:\n" + instructions
	}
	return p
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Wire format (OpenAI chat completions subset).
type (
	chatRequest struct {
		Model     string    `json:"model"`
		Messages  []message `json:"messages"`
		MaxTokens int       `json:"max_tokens"`
		Stream    bool      `json:"stream"`
	}
	message struct {
		Role    string `json:"role"`
		Content any    `json:"content"` // string or []contentPart
	}
	contentPart struct {
		Type     string    `json:"type"`
		Text     string    `json:"text,omitempty"`
		ImageURL *imageURL `json:"image_url,omitempty"`
	}
	imageURL struct {
		URL string `json:"url"`
	}
	chatResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
)
