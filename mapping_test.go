package main

import (
	"testing"
)

func TestExtractString_flat(t *testing.T) {
	data := map[string]any{"key": "value"}
	got, ok := ExtractString(data, "key")
	if !ok || got != "value" {
		t.Errorf("got (%q, %v), want (\"value\", true)", got, ok)
	}
}

func TestExtractString_nested(t *testing.T) {
	data := map[string]any{
		"routing": map[string]any{"target": "homeserver"},
	}
	got, ok := ExtractString(data, "routing.target")
	if !ok || got != "homeserver" {
		t.Errorf("got (%q, %v), want (\"homeserver\", true)", got, ok)
	}
}

func TestExtractString_missing(t *testing.T) {
	data := map[string]any{"other": "x"}
	_, ok := ExtractString(data, "key")
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

func TestExtractString_wrongType(t *testing.T) {
	data := map[string]any{"key": 42}
	_, ok := ExtractString(data, "key")
	if ok {
		t.Error("expected ok=false for non-string value")
	}
}

func TestExtractString_deeplyNested(t *testing.T) {
	data := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": "deep",
			},
		},
	}
	got, ok := ExtractString(data, "a.b.c")
	if !ok || got != "deep" {
		t.Errorf("got (%q, %v), want (\"deep\", true)", got, ok)
	}
}

func TestExtractBool(t *testing.T) {
	data := map[string]any{"flag": true}
	got, ok := ExtractBool(data, "flag")
	if !ok || !got {
		t.Errorf("got (%v, %v), want (true, true)", got, ok)
	}
}

func TestApplyMapping_success(t *testing.T) {
	raw := []byte(`{"output":"Hello","target":"bridge:whatsapp","target_room":"!abc:matrix.org"}`)
	mapping := map[string]string{
		"body":        "output",
		"destination": "target",
		"room_id":     "target_room",
	}
	msg, err := ApplyMapping(raw, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Body != "Hello" {
		t.Errorf("body: got %q, want %q", msg.Body, "Hello")
	}
	if msg.Destination != "bridge:whatsapp" {
		t.Errorf("destination: got %q, want %q", msg.Destination, "bridge:whatsapp")
	}
	if msg.RoomID != "!abc:matrix.org" {
		t.Errorf("room_id: got %q, want %q", msg.RoomID, "!abc:matrix.org")
	}
}

func TestApplyMapping_missingBody(t *testing.T) {
	raw := []byte(`{"target":"homeserver","target_room":"!abc:matrix.org"}`)
	mapping := map[string]string{
		"body":        "output",
		"destination": "target",
	}
	_, err := ApplyMapping(raw, mapping)
	if err == nil {
		t.Error("expected error for missing body")
	}
}

func TestApplyMapping_missingDestination(t *testing.T) {
	raw := []byte(`{"output":"hello"}`)
	mapping := map[string]string{
		"body":        "output",
		"destination": "target",
	}
	_, err := ApplyMapping(raw, mapping)
	if err == nil {
		t.Error("expected error for missing destination")
	}
}

func TestApplyMapping_invalidJSON(t *testing.T) {
	_, err := ApplyMapping([]byte(`not json`), map[string]string{"body": "b", "destination": "d"})
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestApplyMapping_nestedPaths(t *testing.T) {
	raw := []byte(`{"meta":{"dest":"homeserver"},"payload":{"text":"hi"},"room":"!x:y"}`)
	mapping := map[string]string{
		"body":        "payload.text",
		"destination": "meta.dest",
		"room_id":     "room",
	}
	msg, err := ApplyMapping(raw, mapping)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Body != "hi" || msg.Destination != "homeserver" || msg.RoomID != "!x:y" {
		t.Errorf("unexpected result: %+v", msg)
	}
}
