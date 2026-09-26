// Package centrifugo is a minimal client for the Centrifugo server HTTP API
// (publish/broadcast) plus the types of its RPC proxy protocol.
package centrifugo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	base string
	key  string
	http *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{base: baseURL, key: apiKey, http: &http.Client{Timeout: 3 * time.Second}}
}

func (c *Client) call(ctx context.Context, method string, body any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/"+method, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.key)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("centrifugo %s: status %d: %s", method, resp.StatusCode, data)
	}
	var r struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &r) == nil && r.Error != nil {
		return fmt.Errorf("centrifugo %s: %d %s", method, r.Error.Code, r.Error.Message)
	}
	return nil
}

// Publish sends data to one channel.
func (c *Client) Publish(ctx context.Context, channel string, data any) error {
	return c.call(ctx, "publish", map[string]any{"channel": channel, "data": data})
}

// Broadcast sends the same data to many channels.
func (c *Client) Broadcast(ctx context.Context, channels []string, data any) error {
	if len(channels) == 0 {
		return nil
	}
	return c.call(ctx, "broadcast", map[string]any{"channels": channels, "data": data})
}

// Disconnect closes every connection of a user (kick).
func (c *Client) Disconnect(ctx context.Context, user string) error {
	return c.call(ctx, "disconnect", map[string]any{"user": user, "disconnect": map[string]any{"code": 3503, "reason": "kicked by admin"}})
}

// PersonalChannel is the per-character notification channel. It is attached
// as a server-side subscription in the connection token.
func PersonalChannel(characterID string) string { return "personal:" + characterID }

// NewsChannel carries world-wide announcements (world firsts, etc).
const NewsChannel = "news:world"

// RPCRequest is what Centrifugo POSTs to the RPC proxy endpoint.
type RPCRequest struct {
	Client    string          `json:"client"`
	Transport string          `json:"transport"`
	Protocol  string          `json:"protocol"`
	Encoding  string          `json:"encoding"`
	User      string          `json:"user"`
	Method    string          `json:"method"`
	Data      json.RawMessage `json:"data"`
}

// RPCResponse is the reply expected by Centrifugo.
type RPCResponse struct {
	Result *RPCResult `json:"result,omitempty"`
	Error  *RPCError  `json:"error,omitempty"`
}

type RPCResult struct {
	Data any `json:"data"`
}

type RPCError struct {
	Code    uint32 `json:"code"`
	Message string `json:"message"`
}
