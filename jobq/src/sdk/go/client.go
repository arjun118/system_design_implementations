package jobq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

func (c *Client) SubmitJob(ctx context.Context, req SubmitJobRequest) (uuid.UUID, error) {
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

func (c *Client) GetJob(ctx context.Context, ID uuid.UUID) (*Job, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/jobs/"+ID.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request, %w", err)
	}
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request, %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", httpResp.StatusCode)
	}
	var job Job
	if err := json.NewDecoder(httpResp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("failed to decode job details response: %w", err)
	}
	return &job, nil
}

func (c *Client) GetQueueStats(ctx context.Context, queueName string) (*QueueStats, error) {
	endpoint, err := url.JoinPath(c.baseURL, "v1", "queues", queueName, "stats")
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint specific url, %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request, %w", err)
	}
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request, %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", httpResp.StatusCode)
	}
	var stats QueueStats
	if err := json.NewDecoder(httpResp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode job details response: %w", err)
	}
	return &stats, nil
}

func (c *Client) Claim(ctx context.Context, queues []string, workerID string, leaseDurationSeconds int) (*Job, error) {
	body, err := json.Marshal(ClaimJobRequest{
		Queues:               queues,
		WorkerID:             workerID,
		LeaseDurationSeconds: leaseDurationSeconds,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create request body, %w", err)
	}
	endpoint, err := url.JoinPath(c.baseURL, "v1", "jobs", "claim")
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint specific url, %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request, %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request, %w", err)
	}
	defer httpResp.Body.Close()
	switch httpResp.StatusCode {
	case http.StatusNoContent:
		return nil, ErrNoJobAvailable
	case http.StatusOK:
		var job Job
		if err := json.NewDecoder(httpResp.Body).Decode(&job); err != nil {
			return nil, fmt.Errorf("failed to decode job response: %w", err)
		}
		return &job, nil
	default:
		return nil, fmt.Errorf("unexpected status code: %d", httpResp.StatusCode)
	}

}

func (c *Client) Ack(ctx context.Context, jobID uuid.UUID, leaseToken int) error {
	body, err := json.Marshal(AckRequest{
		LeaseToken: leaseToken,
	})
	if err != nil {
		return fmt.Errorf("failed to create request body, %w", err)
	}
	endpoint, err := url.JoinPath(c.baseURL, "v1", "jobs", jobID.String(), "ack")
	if err != nil {
		return fmt.Errorf("failed to parse endpoint specific url, %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create http request, %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request, %w", err)
	}
	defer httpResp.Body.Close()

	switch httpResp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrJobNotFound
	case http.StatusConflict:
		return ErrStaleLeaseToken
	default:
		return fmt.Errorf("unexpected status code: %d", httpResp.StatusCode)
	}
}

func (c *Client) NAck(ctx context.Context, jobID uuid.UUID, leaseToken int, workerError error, permanent bool) error {
	errMsg := ""
	if workerError != nil {
		errMsg = workerError.Error()
	}
	body, err := json.Marshal(NAckRequest{
		LeaseToken: leaseToken,
		Error:      errMsg,
		Permanent:  permanent,
	})
	if err != nil {
		return fmt.Errorf("failed to create request body, %w", err)
	}
	endpoint, err := url.JoinPath(c.baseURL, "v1", "jobs", jobID.String(), "nack")
	if err != nil {
		return fmt.Errorf("failed to parse endpoint specific url, %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create http request, %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request, %w", err)
	}
	defer httpResp.Body.Close()
	switch httpResp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrJobNotFound
	case http.StatusConflict:
		return ErrStaleLeaseToken
	default:
		return fmt.Errorf("unexpected status code: %d", httpResp.StatusCode)
	}
}

func (c *Client) Heartbeat(ctx context.Context, jobID uuid.UUID, leaseToken int, workerID string, leaseDuration int) error {
	body, err := json.Marshal(HeartBeatRequest{
		WorkerID:             workerID,
		LeaseToken:           leaseToken,
		LeaseDurationSeconds: leaseDuration,
	})
	if err != nil {
		return fmt.Errorf("failed to create request body, %w", err)
	}
	endpoint, err := url.JoinPath(c.baseURL, "v1", "jobs", jobID.String(), "heartbeat")
	if err != nil {
		return fmt.Errorf("failed to parse endpoint specific url, %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create http request, %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request, %w", err)
	}
	defer httpResp.Body.Close()
	switch httpResp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrJobNotFound
	case http.StatusConflict:
		return ErrStaleLeaseToken
	default:
		return fmt.Errorf("unexpected response code: %v", httpResp.StatusCode)
	}
}

func (c *Client) RetryDeadJob(ctx context.Context, jobID uuid.UUID) error {
	endpoint, err := url.JoinPath(c.baseURL, "v1", "jobs", jobID.String(), "retry")
	if err != nil {
		return fmt.Errorf("failed to parse retry endpoint url: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create retry request: %w", err)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send retry request: %w", err)
	}
	defer httpResp.Body.Close()

	switch httpResp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrJobNotFound
	case http.StatusConflict:
		return ErrJobNotDead
	default:
		return fmt.Errorf("unexpected status code: %d", httpResp.StatusCode)
	}
}

func (c *Client) GetDeadLetteredJobs(ctx context.Context, queueName string) ([]*Job, error) {
	endpoint, err := url.JoinPath(c.baseURL, "v1", "queues", queueName, "dead-letters")
	if err != nil {
		return nil, fmt.Errorf("failed to parse dead-letters endpoint url: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create dead-letters request: %w", err)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send dead-letters request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", httpResp.StatusCode)
	}

	var jobs []*Job
	if err := json.NewDecoder(httpResp.Body).Decode(&jobs); err != nil {
		return nil, fmt.Errorf("failed to decode dead-lettered jobs: %w", err)
	}
	return jobs, nil
}
