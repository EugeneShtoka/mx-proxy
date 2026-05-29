package main

import "fmt"

type Transport interface {
	// SendRecv sends payload to the processor and blocks until a response is
	// received. Concurrent calls are safe; responses are correlated by ID.
	SendRecv(payload []byte) ([]byte, error)
	Close() error
}

func NewTransport(cfg ProcessorConfig) (Transport, error) {
	switch cfg.Transport {
	case "unix":
		return newUnixTransport(cfg.Endpoint), nil
	case "websocket":
		return newWebSocketTransport(cfg.Endpoint), nil
	case "http":
		return newHTTPTransport(cfg.Endpoint), nil
	default:
		return nil, fmt.Errorf("unknown transport: %s", cfg.Transport)
	}
}
