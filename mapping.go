package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ExtractString(data map[string]any, path string) (string, bool) {
	v, ok := extractAny(data, path)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func ExtractBool(data map[string]any, path string) (bool, bool) {
	v, ok := extractAny(data, path)
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func extractAny(data map[string]any, path string) (any, bool) {
	parts := strings.SplitN(path, ".", 2)
	val, ok := data[parts[0]]
	if !ok {
		return nil, false
	}
	if len(parts) == 1 {
		return val, true
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return nil, false
	}
	return extractAny(nested, parts[1])
}

type MappedMessage struct {
	Body            string
	ReplyFallback   string         // "> quote" prefix to prepend when reconstructing a reply body
	Destination     string
	RoomID          string
	Sender          string
	OriginalContent map[string]any // original event content, preserved on re-injection
}

func ApplyMapping(raw []byte, mapping map[string]string) (MappedMessage, error) {
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return MappedMessage{}, fmt.Errorf("unmarshal processor message: %w", err)
	}

	var msg MappedMessage
	for _, f := range []struct {
		key string
		dst *string
	}{
		{"body", &msg.Body},
		{"reply_fallback", &msg.ReplyFallback},
		{"destination", &msg.Destination},
		{"room_id", &msg.RoomID},
		{"sender", &msg.Sender},
	} {
		if path, ok := mapping[f.key]; ok {
			*f.dst, _ = ExtractString(data, path)
		}
	}

	if msg.Body == "" {
		return MappedMessage{}, fmt.Errorf("missing body in processor message")
	}
	if msg.Destination == "" {
		return MappedMessage{}, fmt.Errorf("missing destination in processor message")
	}

	return msg, nil
}
