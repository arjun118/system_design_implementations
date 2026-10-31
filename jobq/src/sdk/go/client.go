package jobq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(BaseURL string) *Client {
	return &Client{
		baseURL: BaseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) SubmitJob(ctx context.Context, req SubmitRequest) (uuid.UUID, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to marshall submit request paylaod, %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/jobs", bytes.NewReader(body))

	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create http req, %w", err)

	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to send request, %w", err)

	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusCreated {
		return uuid.Nil, fmt.Errorf("unexpected status code: %d", httpResp.StatusCode)
	}
	var jobCreationResponse AcceptJobResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&jobCreationResponse); err != nil {
		return uuid.Nil, fmt.Errorf("failed to decode job creation response: %w", err)
	}
	return jobCreationResponse.ID, nil
}

func (c *Client) GetJob(ctx context.Context, ID uuid.UUID) {

}
