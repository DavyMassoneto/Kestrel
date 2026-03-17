package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
)

const anthropicVersion = "2023-06-01"

// Client implements ChatSender and ChatStreamer for the Claude API.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new Claude API client.
func NewClient(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient}
}

// SendChat sends a synchronous chat request to the Claude API.
func (c *Client) SendChat(ctx context.Context, creds vo.ProviderCredentials, request *vo.ChatRequest) (*vo.ChatResponse, error) {
	claudeReq := DomainToClaudeRequest(request)

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("claude: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, creds.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("claude: failed to create request: %w", err)
	}

	setHeaders(httpReq, creds)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("claude: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("claude: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, NewProviderError(resp.StatusCode, string(respBody))
	}

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return nil, fmt.Errorf("claude: failed to parse response: %w", err)
	}

	return ClaudeResponseToDomain(claudeResp), nil
}

// StreamChat sends a streaming chat request to the Claude API and returns a channel of StreamEvents.
func (c *Client) StreamChat(ctx context.Context, creds vo.ProviderCredentials, request *vo.ChatRequest) (<-chan vo.StreamEvent, error) {
	claudeReq := DomainToClaudeStreamRequest(request)

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("claude: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, creds.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("claude: failed to create request: %w", err)
	}

	setHeaders(httpReq, creds)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("claude: request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, NewProviderError(resp.StatusCode, string(respBody))
	}

	ch := ReadSSE(ctx, resp.Body)
	return ch, nil
}

func setHeaders(req *http.Request, creds vo.ProviderCredentials) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", creds.APIKey.Value())
	req.Header.Set("anthropic-version", anthropicVersion)
}
