package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
)

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
