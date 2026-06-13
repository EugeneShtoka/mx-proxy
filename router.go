package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

var (
	txnCounter atomic.Int64
	txnEpoch   = time.Now().UnixMilli()
)

func nextTxnID() string {
	return fmt.Sprintf("%d-%d", txnEpoch, txnCounter.Add(1))
}

type Router struct {
	cfg    *Config
	client *http.Client
	tokens sync.Map // sender (string) → token (string)
}

func NewRouter(cfg *Config) *Router {
	return &Router{cfg: cfg, client: &http.Client{}}
}

// CacheToken stores the Bearer token associated with a sender so it can be
// reused when the router re-injects a message to the homeserver on their behalf.
func (r *Router) CacheToken(sender, token string) {
	if token != "" {
		r.tokens.Store(sender, token)
	}
}

// Route delivers msg to the homeserver and returns the new Matrix event_id.
func (r *Router) Route(msg MappedMessage) (string, error) {
	return r.routeToHomeserver(msg)
}

func (r *Router) routeToHomeserver(msg MappedMessage) (string, error) {
	txn := nextTxnID()
	u := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/send/m.room.message/%s",
		r.cfg.Upstream.Homeserver, url.PathEscape(msg.RoomID), txn)
	if msg.Sender != "" {
		u += "?user_id=" + url.QueryEscape(msg.Sender)
	}

	content := mergeContent(msg.OriginalContent, msg.Body)
	body, _ := json.Marshal(content)

	req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("route to homeserver: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	r.setAuth(req, msg.Sender)

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("route to homeserver: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("homeserver returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("homeserver response not JSON: %w", err)
	}

	log.Printf("router: delivered to homeserver room %s sender %s (status %d event_id %s)",
		msg.RoomID, msg.Sender, resp.StatusCode, result.EventID)

	return result.EventID, nil
}

func (r *Router) routeToBridge(name string, msg MappedMessage) (string, error) {
	bridge := r.cfg.bridgeByName(name)
	if bridge == nil {
		log.Printf("router: unknown bridge %q — dropping", name)
		return "", nil
	}

	txn := nextTxnID()
	eventID := fmt.Sprintf("$mx-proxy-%s", txn)
	u := fmt.Sprintf("%s/_matrix/app/v1/transactions/%s", bridge.URL, txn)

	event := map[string]any{
		"type":             "m.room.message",
		"room_id":          msg.RoomID,
		"sender":           msg.Sender,
		"event_id":         eventID,
		"origin_server_ts": time.Now().UnixMilli(),
		"content":          mergeContent(msg.OriginalContent, msg.Body),
	}
	body, _ := json.Marshal(map[string]any{"events": []any{event}})

	req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("route to bridge %s: build request: %w", name, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bridge.HSToken)

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("route to bridge %s: %w", name, err)
	}
	resp.Body.Close()
	log.Printf("router: delivered to bridge %s room %s sender %s (status %d)", name, msg.RoomID, msg.Sender, resp.StatusCode)
	return eventID, nil
}


// mergeContent builds the re-injected event content: starts from the original
// content (preserving all fields like fi.mau.double_puppet_source, m.relates_to,
// m.mentions, etc.), overrides body with the processed value, and marks the
// event as forwarded so the proxy won't re-intercept it.
func mergeContent(original map[string]any, body string) map[string]any {
	out := make(map[string]any, len(original)+2)
	for k, v := range original {
		out[k] = v
	}
	out["body"] = body
	return out
}

func (r *Router) setAuth(req *http.Request, sender string) {
	if tok, ok := r.tokens.Load(sender); ok {
		req.Header.Set("Authorization", "Bearer "+tok.(string))
	} else if r.cfg.Upstream.ASToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.Upstream.ASToken)
	}
}
