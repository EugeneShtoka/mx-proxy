package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

type httpTransport struct {
	endpoint string
	client   *http.Client
}

func newHTTPTransport(endpoint string) *httpTransport {
	return &httpTransport{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *httpTransport) SendRecv(payload []byte) ([]byte, error) {
	resp, err := t.client.Post(t.endpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("http transport: post: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http transport: read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http transport: status %d", resp.StatusCode)
	}
	return body, nil
}

func (t *httpTransport) Close() error {
	return nil
}
