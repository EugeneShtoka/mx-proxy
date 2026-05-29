package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var reqCounter atomic.Int64

func newReqID() string {
	return fmt.Sprintf("%d", reqCounter.Add(1))
}

type unixTransport struct {
	endpoint string

	connMu  sync.Mutex
	conn    net.Conn
	writeMu sync.Mutex // serializes writes; reads are handled by the single readLoop goroutine

	pendingMu sync.Mutex
	pending   map[string]chan []byte
}

func newUnixTransport(endpoint string) *unixTransport {
	t := &unixTransport{
		endpoint: endpoint,
		pending:  make(map[string]chan []byte),
	}
	go t.connectLoop()
	return t
}

func (t *unixTransport) connectLoop() {
	backoff := time.Second
	for {
		conn, err := net.DialTimeout("unix", t.endpoint, 5*time.Second)
		if err != nil {
			log.Printf("unix transport: connect %s: %v — retry in %s", t.endpoint, err, backoff)
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		log.Printf("unix transport: connected to %s", t.endpoint)

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

		log.Printf("unix transport: disconnected from %s — reconnecting", t.endpoint)
	}
}

func (t *unixTransport) readLoop(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal(line, &resp); err != nil {
			log.Printf("unix transport: unparseable response: %v", err)
			continue
		}
		id, _ := resp["id"].(string)
		if id == "" {
			log.Printf("unix transport: response missing id — dropping")
			continue
		}
		t.pendingMu.Lock()
		ch, ok := t.pending[id]
		if ok {
			delete(t.pending, id)
		}
		t.pendingMu.Unlock()

		if ok {
			cp := make([]byte, len(line))
			copy(cp, line)
			ch <- cp
		}
	}
}

func (t *unixTransport) SendRecv(payload []byte) ([]byte, error) {
	id := newReqID()

	// Inject correlation id into the payload JSON.
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil, fmt.Errorf("unix transport: invalid payload: %w", err)
	}
	obj["id"] = id
	stamped, _ := json.Marshal(obj)

	respCh := make(chan []byte, 1)

	t.connMu.Lock()
	conn := t.conn
	t.connMu.Unlock()
	if conn == nil {
		return nil, fmt.Errorf("unix transport: not connected")
	}

	t.pendingMu.Lock()
	t.pending[id] = respCh
	t.pendingMu.Unlock()

	t.writeMu.Lock()
	_, err := conn.Write(append(stamped, '\n'))
	t.writeMu.Unlock()

	if err != nil {
		t.pendingMu.Lock()
		delete(t.pending, id)
		t.pendingMu.Unlock()
		return nil, fmt.Errorf("unix transport: write: %w", err)
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return nil, fmt.Errorf("unix transport: connection closed while waiting for response")
		}
		return resp, nil
	case <-time.After(30 * time.Second):
		t.pendingMu.Lock()
		delete(t.pending, id)
		t.pendingMu.Unlock()
		return nil, fmt.Errorf("unix transport: timeout waiting for response to %s", id)
	}
}

func (t *unixTransport) Close() error {
	t.connMu.Lock()
	defer t.connMu.Unlock()
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}
