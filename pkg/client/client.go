package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sandforge/sandforge/pkg/api"
)

// ── Types ───────────────────────────────────────────────────────────────────

// Client talks to the Sandforge control plane over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Sandbox is a handle to a created sandbox.
type Sandbox struct {
	ID string
}

// createRequest mirrors the control plane's POST /v1/sandboxes body.
type createRequest struct {
	ID   string          `json:"id"`
	Spec api.SandboxSpec `json:"spec"`
}

// createResponse mirrors the control plane's create response.
type createResponse struct {
	ID string `json:"id"`
}

// statusResponse mirrors GET /v1/sandboxes/{id}.
type statusResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// apiError represents the error JSON the control plane returns.
type apiError struct {
	Error string `json:"error"`
}

// ── Constructor ─────────────────────────────────────────────────────────────

// NewClient returns a Client pointed at baseURL (e.g. "http://localhost:8080").
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ── Public API ──────────────────────────────────────────────────────────────

// CreateSandbox provisions a new sandbox from spec and returns a handle.
// The SDK generates a unique ID for the caller.
func (c *Client) CreateSandbox(ctx context.Context, spec api.SandboxSpec) (*Sandbox, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate id: %w", err)
	}

	var resp createResponse
	if err := c.do(ctx, "POST", "/v1/sandboxes", createRequest{ID: id, Spec: spec}, &resp); err != nil {
		return nil, err
	}
	return &Sandbox{ID: resp.ID}, nil
}

// Exec runs a command inside the sandbox identified by id and returns its result
// (exit code, stdout, stderr, and any artifacts).
func (c *Client) Exec(ctx context.Context, id string, req api.ExecRequest) (*api.ExecResult, error) {
	var result api.ExecResult
	if err := c.do(ctx, "POST", "/v1/sandboxes/"+id+"/exec", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetStatus returns the current lifecycle state of a sandbox.
func (c *Client) GetStatus(ctx context.Context, id string) (string, error) {
	var resp statusResponse
	if err := c.do(ctx, "GET", "/v1/sandboxes/"+id, nil, &resp); err != nil {
		return "", err
	}
	return resp.State, nil
}

// Destroy tears down the sandbox with the given id.
func (c *Client) Destroy(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/sandboxes/"+id, nil, nil)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// do is the core HTTP helper used by every public method.
// It marshals body to JSON, makes the request, checks the status code,
// and unmarshals the response into out (pass nil if no response body expected).
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	fullURL := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return newAPIError(resp.StatusCode, respBody)
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

func newAPIError(statusCode int, body []byte) error {
	var e apiError
	if err := json.Unmarshal(body, &e); err != nil || e.Error == "" {
		return fmt.Errorf("HTTP %d", statusCode)
	}
	return fmt.Errorf("HTTP %d: %s", statusCode, e.Error)
}

// generateID returns a random hex sandbox identifier.
func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sbx-" + hex.EncodeToString(b), nil
}
