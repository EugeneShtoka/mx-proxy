package main

import "github.com/gorilla/websocket"

type wsConn struct {
	conn *websocket.Conn
}

func (c *wsConn) ReadMessage() ([]byte, error) {
	_, msg, err := c.conn.ReadMessage()
	return msg, err
}

func (c *wsConn) WriteMessage(msg []byte) error {
	return c.conn.WriteMessage(websocket.TextMessage, msg)
}

func (c *wsConn) Close() error { return c.conn.Close() }

func dialWS(endpoint string) (connAdapter, error) {
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if err != nil {
		return nil, err
	}
	return &wsConn{conn: conn}, nil
}

func newWebSocketTransport(endpoint string) *baseTransport {
	return newBaseTransport("websocket", endpoint, dialWS)
}
