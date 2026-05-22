package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SpendLog represents a single entry from LiteLLM /spend/logs.
type SpendLog struct {
	RequestID       string                 `json:"request_id"`
	CallType        string                 `json:"call_type"`
	Model           string                 `json:"model"`
	PromptTokens    int                    `json:"prompt_tokens"`
	CompletionTokens int                   `json:"completion_tokens"`
	Spend           float64                `json:"spend"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	StartTime       string                 `json:"startTime,omitempty"`
	EndTime         string                 `json:"endTime,omitempty"`
}

// LiteLLMClient is the interface for fetching spend logs from LiteLLM.
// Interface-based design allows mock HTTP server for testing.
type LiteLLMClient interface {
	FetchSpendLogs(ctx context.Context, startDate, endDate time.Time) ([]SpendLog, error)
}

// HTTPLiteLLMClient implements LiteLLMClient using a real HTTP client.
type HTTPLiteLLMClient struct {
	endpoint   string
	httpClient *http.Client
}

// NewHTTPLiteLLMClient creates a new HTTP-based LiteLLM client.
func NewHTTPLiteLLMClient(endpoint string) *HTTPLiteLLMClient {
	return &HTTPLiteLLMClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchSpendLogs fetches spend logs from LiteLLM /spend/logs.
func (c *HTTPLiteLLMClient) FetchSpendLogs(ctx context.Context, startDate, endDate time.Time) ([]SpendLog, error) {
	url := fmt.Sprintf("%s/spend/logs?summarize=false&start_date=%s&end_date=%s",
		c.endpoint,
		startDate.Format(time.RFC3339),
		endDate.Format(time.RFC3339),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("audit: litellm request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("audit: litellm fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("audit: litellm returned %d: %s", resp.StatusCode, string(body))
	}

	var logs []SpendLog
	if err := json.NewDecoder(resp.Body).Decode(&logs); err != nil {
		return nil, fmt.Errorf("audit: litellm decode: %w", err)
	}

	return logs, nil
}
