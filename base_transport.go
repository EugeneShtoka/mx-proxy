package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

func newReqID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("mx-proxy-%x", b)
}

// connAdapter abstracts the wire differences between transport types.
type connAdapter interface {
	ReadMessage() ([]byte, error)
	WriteMessage([]byte) error
	Close() error
}

type baseTransport struct {
	name      string
	endpoint  string
	dial      func(string) (connAdapter, error)
	connMu    sync.Mutex
	conn      connAdapter
	writeMu   sync.Mutex
	pendingMu sync.Mutex
	pending   map[string]chan []byte
}

func newBaseTransport(name, endpoint string, dial func(string) (connAdapter, error)) *baseTransport {
	t := &baseTransport{
		name:     name,
		endpoint: endpoint,
		dial:     dial,
		pending:  make(map[string]chan []byte),
	}
	go t.connectLoop()
	return t
}

func (t *baseTransport) connectLoop() {
	backoff := time.Second
	for {
		conn, err := t.dial(t.endpoint)
		if err != nil {
			log.Printf("%s transport: dial %s: %v — retry in %s", t.name, t.endpoint, err, backoff)
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		log.Printf("%s transport: connected to %s", t.name, t.endpoint)

		t.connMu.Lock()
		t.conn = conn
		t.connMu.Unlock()

		t.readLoop(conn)

		t.connMu.Lock()
		t.conn = nil
		t.connMu.Unlock()

		// Fail all in-flight requests.
		t.pendingMu.Lock()
		for id, ch := range t.pending {
			close(ch)
			delete(t.pending, id)
		}
		t.pendingMu.Unlock()

		log.Printf("%s transport: disconnected from %s — reconnecting", t.name, t.endpoint)
	}
}

func (t *baseTransport) readLoop(conn connAdapter) {
	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if len(msg) == 0 {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal(msg, &resp); err != nil {
			log.Printf("%s transport: unparseable response: %v", t.name, err)
			continue
		}
		id, _ := resp["id"].(string)
		if id == "" {
			log.Printf("%s transport: response missing id — dropping", t.name)
			continue
		}
		t.pendingMu.Lock()
		ch, ok := t.pending[id]
		if ok {
			delete(t.pending, id)
		}
		t.pendingMu.Unlock()

		if ok {
			ch <- msg
		}
	}
}

func (t *baseTransport) SendRecv(payload []byte) ([]byte, error) {
	id := newReqID()

	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil, fmt.Errorf("%s transport: invalid payload: %w", t.name, err)
	}
	obj["id"] = id
	stamped, _ := json.Marshal(obj)

	respCh := make(chan []byte, 1)

	t.connMu.Lock()
	conn := t.conn
	t.connMu.Unlock()
	if conn == nil {
		return nil, fmt.Errorf("%s transport: not connected", t.name)
	}

	t.pendingMu.Lock()
	t.pending[id] = respCh
	t.pendingMu.Unlock()

	t.writeMu.Lock()
	err := conn.WriteMessage(stamped)
	t.writeMu.Unlock()

	if err != nil {
		t.pendingMu.Lock()
		delete(t.pending, id)
		t.pendingMu.Unlock()
		return nil, fmt.Errorf("%s transport: write: %w", t.name, err)
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return nil, fmt.Errorf("%s transport: connection closed while waiting for response", t.name)
		}
		return resp, nil
	case <-time.After(30 * time.Second):
		t.pendingMu.Lock()
		delete(t.pending, id)
		t.pendingMu.Unlock()
		return nil, fmt.Errorf("%s transport: timeout waiting for response to %s", t.name, id)
	}
}

func (t *baseTransport) Close() error {
	t.connMu.Lock()
	defer t.connMu.Unlock()
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}
