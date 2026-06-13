package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// forwardOrAck forwards the request to targetURL. If the upstream returns 5xx
// (e.g. a deleted room), it writes a synthetic 200 with a dummy event_id so
// the client stops retrying an undeliverable message.
func forwardOrAck(w http.ResponseWriter, r *http.Request, targetURL string, body []byte) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL+r.RequestURI, bytes.NewReader(body))
	if err != nil {
		log.Printf("forward: build request: %v", err)
		synthetic200(w)
		return
	}
	for k, vals := range r.Header {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	req.ContentLength = int64(len(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("forwardOrAck: upstream error: %v — synthesizing 200", err)
		synthetic200(w)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		log.Printf("forwardOrAck: upstream returned %d — synthesizing 200 to unblock client", resp.StatusCode)
		io.Copy(io.Discard, resp.Body)
		synthetic200(w)
		return
	}

	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func synthetic200(w http.ResponseWriter) {
	fakeID := fmt.Sprintf("$mx-proxy-ack-%d", time.Now().UnixMilli())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"event_id": fakeID})
}

func forward(w http.ResponseWriter, r *http.Request, targetURL string, body []byte) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL+r.RequestURI, bytes.NewReader(body))
	if err != nil {
		log.Printf("forward: build request: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	for k, vals := range r.Header {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	req.ContentLength = int64(len(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("forward: do request: %v", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
