package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type wsTransport struct {
	endpoint string

	connMu  sync.Mutex
	conn    *websocket.Conn
	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan []byte
}

func newWebSocketTransport(endpoint string) *wsTransport {
	t := &wsTransport{
		endpoint: endpoint,
		pending:  make(map[string]chan []byte),
	}
	go t.connectLoop()
	return t
}

func (t *wsTransport) connectLoop() {
	backoff := time.Second
	for {
		conn, _, err := websocket.DefaultDialer.Dial(t.endpoint, nil)
		if err != nil {
			log.Printf("websocket transport: dial %s: %v — retry in %s", t.endpoint, err, backoff)
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		log.Printf("websocket transport: connected to %s", t.endpoint)

		t.connMu.Lock()
		t.conn = conn
		t.connMu.Unlock()

		t.readLoop(conn)

		t.connMu.Lock()
		t.conn = nil
		t.connMu.Unlock()

		t.pendingMu.Lock()
		for id, ch := range t.pending {
			close(ch)
			delete(t.pending, id)
		}
		t.pendingMu.Unlock()

		log.Printf("websocket transport: disconnected from %s — reconnecting", t.endpoint)
	}
}

func (t *wsTransport) readLoop(conn *websocket.Conn) {
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if len(msg) == 0 {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal(msg, &resp); err != nil {
			log.Printf("websocket transport: unparseable response: %v", err)
			continue
		}
		id, _ := resp["id"].(string)
		if id == "" {
			log.Printf("websocket transport: response missing id — dropping")
			continue
		}
		t.pendingMu.Lock()
		ch, ok := t.pending[id]
		if ok {
			delete(t.pending, id)
		}
		t.pendingMu.Unlock()

		if ok {
			cp := make([]byte, len(msg))
			copy(cp, msg)
			ch <- cp
		}
	}
}

func (t *wsTransport) SendRecv(payload []byte) ([]byte, error) {
	id := newReqID()

	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil, fmt.Errorf("websocket transport: invalid payload: %w", err)
	}
	obj["id"] = id
	stamped, _ := json.Marshal(obj)

	respCh := make(chan []byte, 1)

	t.connMu.Lock()
	conn := t.conn
	t.connMu.Unlock()
	if conn == nil {
		return nil, fmt.Errorf("websocket transport: not connected")
	}

	t.pendingMu.Lock()
	t.pending[id] = respCh
	t.pendingMu.Unlock()

	t.writeMu.Lock()
	err := conn.WriteMessage(websocket.TextMessage, stamped)
	t.writeMu.Unlock()

	if err != nil {
		t.pendingMu.Lock()
		delete(t.pending, id)
		t.pendingMu.Unlock()
		return nil, fmt.Errorf("websocket transport: write: %w", err)
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return nil, fmt.Errorf("websocket transport: connection closed while waiting for response")
		}
		return resp, nil
	case <-time.After(30 * time.Second):
		t.pendingMu.Lock()
		delete(t.pending, id)
		t.pendingMu.Unlock()
		return nil, fmt.Errorf("websocket transport: timeout waiting for response to %s", id)
	}
}

func (t *wsTransport) Close() error {
	t.connMu.Lock()
	defer t.connMu.Unlock()
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}
