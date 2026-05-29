package main

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

type unixConn struct {
	conn    net.Conn
	scanner *bufio.Scanner
}

func (c *unixConn) ReadMessage() ([]byte, error) {
	if !c.scanner.Scan() {
		return nil, fmt.Errorf("connection closed")
	}
	b := c.scanner.Bytes()
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp, nil
}

func (c *unixConn) WriteMessage(msg []byte) error {
	_, err := c.conn.Write(append(msg, '\n'))
	return err
}

func (c *unixConn) Close() error { return c.conn.Close() }

func dialUnix(endpoint string) (connAdapter, error) {
	conn, err := net.DialTimeout("unix", endpoint, 5*time.Second)
	if err != nil {
		return nil, err
	}
	return &unixConn{conn: conn, scanner: bufio.NewScanner(conn)}, nil
}

func newUnixTransport(endpoint string) *baseTransport {
	return newBaseTransport("unix", endpoint, dialUnix)
}
