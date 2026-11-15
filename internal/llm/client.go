package llm

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

type Client struct {
	baseURL string
	model   string
	apiKey  string
	http    *http.Client
}

func New(
	baseURL, model, apiKey string,
	timeout time.Duration,
) *Client {
	trimBaseURL := strings.TrimRight(baseURL, "/")

	return &Client{
		baseURL: trimBaseURL,
		model:   model,
		apiKey:  apiKey,
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Complete(
	ctx context.Context,
	system, user string,
) (string, error) {
	msgs := []message{
		{
			Role:    "system",
			Content: system,
		},
		{
			Role:    "user",
			Content: user,
		},
	}

	req := chatRequest{
		Model:       c.model,
		Messages:    msgs,
		Temperature: 0.2,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf(
			"marshal request: %w",
			err,
		)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader((body)))
	if err != nil {
		return "", fmt.Errorf(
			"new request: %w",
			err,
		)
	}

	request.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return "", fmt.Errorf(
			"send request: %w",
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return "", fmt.Errorf(
			"llm status %d: %s",
			response.StatusCode,
			strings.TrimSpace(string(text)),
		)
	}

	var resp chatResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return "", fmt.Errorf(
			"decode response: %w",
			err,
		)
	}

	if len(resp.Choices) == 0 {
		return "", ErrEmptyResponse
	}

	answer := strings.TrimSpace(resp.Choices[0].Message.Content)
	if answer == "" {
		return "", ErrEmptyResponse
	}

	return answer, nil
}
